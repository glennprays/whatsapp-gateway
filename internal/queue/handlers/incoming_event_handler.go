package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	customLog "github.com/glennprays/log"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/queue"
)

type IncomingEventHandler struct {
	Repository     whatsapp.WhatsAppRepository
	Publisher      *queue.RabbitMQQueue
	Logger         *customLog.Logger
	MediaDownloader whatsapp.MediaDownloader
	Clients        map[string]*whatsmeow.Client
}

func (h *IncomingEventHandler) Handle(ctx context.Context, body []byte, headers amqp.Table) error {
	traceID := queue.GetTraceIDWorkerProcess(headers)
	// Unmarshal incoming event message
	var eventMsg domainQueue.IncomingEventMessage
	if err := json.Unmarshal(body, &eventMsg); err != nil {
		return fmt.Errorf("failed to unmarshal incoming event: %w", err)
	}

	// Unmarshal WhatsApp event
	var waEvent events.Message
	if err := json.Unmarshal(eventMsg.Event, &waEvent); err != nil {
		return fmt.Errorf("failed to unmarshal WhatsApp event: %w", err)
	}

	// Check for duplicate message
	// TODO: Implement duplicate detection if needed

	// Get client for JID resolution
	var client *whatsmeow.Client
	if h.Clients != nil {
		client = h.Clients[eventMsg.PhoneNumber]
	}

	// Build webhook payload with client
	payload := buildWebhookPayload(&waEvent, h.MediaDownloader, traceID, eventMsg.PhoneNumber, client)

	// Fetch webhook config from database using JID
	webhook, err := h.Repository.GetWebhook(ctx, eventMsg.JID)
	if err != nil {
		return fmt.Errorf("failed to get webhook config: %w", err)
	}

	if webhook == nil || webhook.Url == "" {
		h.Logger.Debug("", fmt.Sprintf("No webhook URL configured for phone %s", eventMsg.PhoneNumber), nil)
		return nil // Not an error, just skip
	}

	// Enqueue to webhook delivery queue
	webhookMsg := domainQueue.WebhookDeliveryMessage{
		WebhookURL: webhook.Url,
		HmacSecret: webhook.HmacSecret,
		Payload:    payload,
		MessageID:  eventMsg.MessageID,
	}

	if err := h.Publisher.PublishWebhookDelivery(ctx, webhookMsg); err != nil {
		return fmt.Errorf("failed to publish webhook delivery: %w", err)
	}

	h.Logger.Debug(traceID, fmt.Sprintf("Queued webhook delivery for message %s", eventMsg.MessageID), nil)
	return nil
}

func buildWebhookPayload(msg *events.Message, mediaDownloader whatsapp.MediaDownloader, traceID string, phoneNumber string, client *whatsmeow.Client) map[string]interface{} {
	payload := map[string]interface{}{
		"event":      string(domainQueue.EventMessageIncoming),
		"message_id": msg.Info.ID,
		"timestamp":  msg.Info.Timestamp.Unix(),
		"from":       whatsapp.ConvertJIDToNonADLID(msg.Info.Sender, msg.Info.Chat, client),
		"chat":       msg.Info.Chat.String(),
		"is_group":   msg.Info.IsGroup,
		"push_name":  msg.Info.PushName,
	}

	if msg.Message == nil {
		return payload
	}

	// Extract media URLs
	var mediaInfo map[string]interface{}

	switch {
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

	case msg.Message.Conversation != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.Conversation

	case msg.Message.ExtendedTextMessage != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.ExtendedTextMessage.Text
	}

	if mediaInfo != nil {
		payload["media"] = mediaInfo
	}

	return payload
}

func downloadMedia(
	mediaDownloader whatsapp.MediaDownloader,
	traceID string,
	phoneNumber string,
	mediaMessage whatsapp.DownloadableMessage,
	mediaType string,
) (string, error) {
	if mediaDownloader == nil {
		return "", nil
	}
	return mediaDownloader.DownloadAndStoreMedia(
		context.Background(),
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
		"sha256":    fmt.Sprintf("%x", imgMsg.GetFileSHA256()),
	}

	// Handle URL based on storage result
	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
		mediaInfo["storage_url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
		mediaInfo["whatsapp_url"] = whatsappURL
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
		"sha256":    fmt.Sprintf("%x", vidMsg.GetFileSHA256()),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
		mediaInfo["storage_url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
		mediaInfo["whatsapp_url"] = whatsappURL
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
		"sha256":    fmt.Sprintf("%x", audioMsg.GetFileSHA256()),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
		mediaInfo["storage_url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
		mediaInfo["whatsapp_url"] = whatsappURL
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
		"sha256":    fmt.Sprintf("%x", docMsg.GetFileSHA256()),
	}

	if storageErr == nil && storageURL != "" {
		mediaInfo["url"] = storageURL
		mediaInfo["storage_url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
		mediaInfo["whatsapp_url"] = whatsappURL
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
		mediaInfo["storage_url"] = storageURL
	} else {
		mediaInfo["url"] = whatsappURL
		mediaInfo["whatsapp_url"] = whatsappURL
	}

	return mediaInfo
}
