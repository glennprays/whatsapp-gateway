package whatsapp

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	customLog "github.com/glennprays/log"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

type (
	handler struct {
		repository      WhatsAppRepository
		sender          *WebhookSender
		queue           domainQueue.MessageQueue
		logger          *customLog.Logger
		mediaDownloader MediaDownloader
		buffers         map[string]*incomingBuffer
		buffersMu       sync.RWMutex
		webhookSem      chan struct{}
	}
	Handler interface {
		HandleEvent(jid string, evt any)
		GetIncomingMessages(phoneNumber string, limit int) []*IncomingMessage
	}
)

const maxConcurrentWebhooks = 50

func NewHandler(repo WhatsAppRepository, sender *WebhookSender, queue domainQueue.MessageQueue, logger *customLog.Logger, mediaDownloader MediaDownloader) Handler {
	return &handler{
		repository:      repo,
		sender:          sender,
		queue:           queue,
		logger:          logger,
		mediaDownloader: mediaDownloader,
		buffers:         make(map[string]*incomingBuffer),
		webhookSem:      make(chan struct{}, maxConcurrentWebhooks),
	}
}

// getOrCreateBuffer returns the per-phone ring buffer, creating it on first
// use. Uses double-checked locking to avoid serializing every read.
func (h *handler) getOrCreateBuffer(phoneNumber string) *incomingBuffer {
	h.buffersMu.RLock()
	b, ok := h.buffers[phoneNumber]
	h.buffersMu.RUnlock()
	if ok {
		return b
	}
	h.buffersMu.Lock()
	defer h.buffersMu.Unlock()
	if b, ok = h.buffers[phoneNumber]; ok {
		return b
	}
	b = newIncomingBuffer()
	h.buffers[phoneNumber] = b
	return b
}

func (h *handler) GetIncomingMessages(phoneNumber string, limit int) []*IncomingMessage {
	return h.getOrCreateBuffer(phoneNumber).Latest(limit)
}

// Handle messages, events, QR
func (h *handler) HandleEvent(phoneNumber string, evt any) {
	// Generate trace ID for this incoming event
	traceID := uuid.New().String()

	// Type switch on event
	switch v := evt.(type) {
	case *events.Message:
		// Skip outgoing messages (sent by us)
		if v.Info.IsFromMe {
			return
		}

		// Get client for webhook lookup
		client := clients.Get(phoneNumber)
		if client == nil || client.Store == nil || client.Store.ID == nil {
			h.logger.Error(traceID, "Client not found for phone "+MaskedPhoneNumber(phoneNumber), nil)
			return
		}

		jid := client.Store.ID.String()

		// Capture into per-session in-memory buffer before any further
		// processing so both queue and direct paths feed the same source.
		if im := toIncomingMessage(v, client); im != nil {
			h.getOrCreateBuffer(phoneNumber).Push(im)
		}

		// Try to publish to queue if enabled
		if h.queue != nil && h.queue.IsHealthy() {
			eventJSON, err := json.Marshal(v)
			if err != nil {
				h.logger.Error(traceID, "Failed to marshal event for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
				// Fallback to direct delivery
				h.asyncDeliverWebhook(traceID, phoneNumber, jid, v)
				return
			}

			err = h.queue.PublishIncomingEvent(context.Background(), domainQueue.IncomingEventMessage{
				TraceID:     traceID,
				PhoneNumber: phoneNumber,
				JID:         jid,
				Event:       eventJSON,
				MessageID:   v.Info.ID,
				Timestamp:   v.Info.Timestamp.Unix(),
			})
			if err != nil {
				h.logger.Error(traceID, "Queue publish failed for "+MaskedPhoneNumber(phoneNumber)+", using direct delivery", nil, customLog.Error(err))
				// Fallback to direct delivery
				h.asyncDeliverWebhook(traceID, phoneNumber, jid, v)
				return
			}

			h.logger.Debug(traceID, "Successfully queued incoming event for "+MaskedPhoneNumber(phoneNumber), nil)
			return
		}

		// Direct mode (or fallback): deliver webhook asynchronously
		go h.deliverWebhook(traceID, phoneNumber, jid, v)
	}
}

func (h *handler) asyncDeliverWebhook(traceID string, phoneNumber string, jid string, msg *events.Message) {
	select {
	case h.webhookSem <- struct{}{}:
		go func() {
			defer func() { <-h.webhookSem }()
			h.deliverWebhook(traceID, phoneNumber, jid, msg)
		}()
	default:
		h.logger.Warn(traceID, "Webhook delivery semaphore full, dropping event for "+MaskedPhoneNumber(phoneNumber), nil)
	}
}

func (h *handler) deliverWebhook(traceID string, phoneNumber string, jid string, msg *events.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fetch webhook config
	webhook, err := h.repository.GetWebhook(ctx, jid)
	if err != nil || webhook == nil || webhook.Url == "" {
		// No webhook configured, skip silently
		return
	}

	// Validate webhook URL to prevent SSRF attacks
	if err := utils.ValidateURL(webhook.Url); err != nil {
		h.logger.Error(traceID, "Invalid webhook URL format, skipping delivery", nil, customLog.Error(err))
		return
	}

	// Get client for JID resolution
	client := clients.Get(phoneNumber)

	// Build payload with client
	payload := buildWebhookPayload(msg, h.mediaDownloader, traceID, phoneNumber, client)

	// Send webhook
	err = h.sender.Send(ctx, webhook.Url, webhook.HmacSecret, payload)
	if err != nil {
		h.logger.Error(traceID, "Failed to deliver webhook for "+MaskedPhoneNumber(phoneNumber)+" to "+webhook.Url, nil, customLog.Error(err))
	} else {
		h.logger.Info(traceID, "Successfully delivered webhook for "+MaskedPhoneNumber(phoneNumber)+" to "+webhook.Url, nil)
	}
}

func buildWebhookPayload(msg *events.Message, mediaDownloader MediaDownloader, traceID string, phoneNumber string, client *whatsmeow.Client) map[string]interface{} {
	payload := map[string]interface{}{
		"event":      string(domainQueue.EventMessageIncoming),
		"message_id": msg.Info.ID,
		"timestamp":  msg.Info.Timestamp.Unix(),
		"from":       ConvertJIDToNonADLID(msg.Info.Sender, msg.Info.Chat, client),
		"chat":       msg.Info.Chat.String(),
		"is_group":   msg.Info.IsGroup,
		"push_name":  msg.Info.PushName,
	}

	// Extract message content based on type
	if msg.Message == nil {
		return payload
	}

	// Extract media URLs
	var mediaInfo map[string]interface{}

	switch {
	case msg.Message.Conversation != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.Conversation

	case msg.Message.ExtendedTextMessage != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.ExtendedTextMessage.Text

	case msg.Message.ImageMessage != nil:
		payload["type"] = "image"
		whatsappURL := msg.Message.ImageMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.ImageMessage, "image")
		mediaInfo = buildImageMediaInfo(msg.Message.ImageMessage, storageURL, whatsappURL, err)

	case msg.Message.VideoMessage != nil:
		payload["type"] = "video"
		whatsappURL := msg.Message.VideoMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.VideoMessage, "video")
		mediaInfo = buildVideoMediaInfo(msg.Message.VideoMessage, storageURL, whatsappURL, err)

	case msg.Message.AudioMessage != nil:
		payload["type"] = "audio"
		whatsappURL := msg.Message.AudioMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.AudioMessage, "audio")
		mediaInfo = buildAudioMediaInfo(msg.Message.AudioMessage, storageURL, whatsappURL, err)

	case msg.Message.DocumentMessage != nil:
		payload["type"] = "document"
		whatsappURL := msg.Message.DocumentMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.DocumentMessage, "document")
		mediaInfo = buildDocumentMediaInfo(msg.Message.DocumentMessage, storageURL, whatsappURL, err)

	case msg.Message.StickerMessage != nil:
		payload["type"] = "sticker"
		whatsappURL := msg.Message.StickerMessage.GetURL()
		storageURL, err := downloadMedia(mediaDownloader, traceID, phoneNumber, msg.Message.StickerMessage, "sticker")
		mediaInfo = buildStickerMediaInfo(msg.Message.StickerMessage, storageURL, whatsappURL, err)

	case msg.Message.ContactMessage != nil:
		payload["type"] = "contact"
		if msg.Message.ContactMessage.DisplayName != nil {
			payload["display_name"] = *msg.Message.ContactMessage.DisplayName
		}

	case msg.Message.LocationMessage != nil:
		payload["type"] = "location"
		if msg.Message.LocationMessage.DegreesLatitude != nil {
			payload["latitude"] = *msg.Message.LocationMessage.DegreesLatitude
		}
		if msg.Message.LocationMessage.DegreesLongitude != nil {
			payload["longitude"] = *msg.Message.LocationMessage.DegreesLongitude
		}

	default:
		payload["type"] = "unknown"
	}

	if mediaInfo != nil {
		payload["media"] = mediaInfo
	}

	return payload
}

func downloadMedia(
	mediaDownloader MediaDownloader,
	traceID string,
	phoneNumber string,
	mediaMessage DownloadableMessage,
	mediaType string,
) (string, error) {
	if mediaDownloader == nil {
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return mediaDownloader.DownloadAndStoreMedia(
		ctx,
		traceID,
		phoneNumber,
		mediaMessage,
		mediaType,
	)
}

func buildImageMediaInfo(
	imgMsg *proto.ImageMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "image",
		"mime_type": imgMsg.GetMimetype(),
		"size":      imgMsg.GetFileLength(),
	}

	// Handle URL based on storage result
	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	if caption := imgMsg.GetCaption(); caption != "" {
		mediaInfo["caption"] = caption
	}

	return mediaInfo
}

func buildVideoMediaInfo(
	vidMsg *proto.VideoMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "video",
		"mime_type": vidMsg.GetMimetype(),
		"size":      vidMsg.GetFileLength(),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	if caption := vidMsg.GetCaption(); caption != "" {
		mediaInfo["caption"] = caption
	}

	return mediaInfo
}

func buildAudioMediaInfo(
	audioMsg *proto.AudioMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "audio",
		"mime_type": audioMsg.GetMimetype(),
		"size":      audioMsg.GetFileLength(),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	return mediaInfo
}

func buildDocumentMediaInfo(
	docMsg *proto.DocumentMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type":      "document",
		"file_name": docMsg.GetFileName(),
		"mime_type": docMsg.GetMimetype(),
		"size":      docMsg.GetFileLength(),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	return mediaInfo
}

func buildStickerMediaInfo(
	stickerMsg *proto.StickerMessage,
	storageURL string,
	whatsappURL string,
	storageErr error,
) map[string]interface{} {
	mediaInfo := map[string]interface{}{
		"type": "sticker",
	}

	if stickerMsg.Mimetype != nil {
		mediaInfo["mime_type"] = *stickerMsg.Mimetype
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
	}

	return mediaInfo
}

// GetWebhookURL helper method for compatibility
func (h *handler) GetWebhookURL(ctx context.Context, jid string) (*string, error) {
	webhook, err := h.repository.GetWebhook(ctx, jid)
	if err != nil || webhook == nil {
		return nil, err
	}
	return &webhook.Url, nil
}
