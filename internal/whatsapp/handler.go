package whatsapp

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	customLog "github.com/glennprays/log"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/metrics"
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
		bufferSize      int
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

func NewHandler(repo WhatsAppRepository, sender *WebhookSender, queue domainQueue.MessageQueue, logger *customLog.Logger, mediaDownloader MediaDownloader, bufferSize int) Handler {
	return &handler{
		repository:      repo,
		sender:          sender,
		queue:           queue,
		logger:          logger,
		mediaDownloader: mediaDownloader,
		bufferSize:      bufferSize,
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
	b = newIncomingBuffer(h.bufferSize)
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
		clients.SetJID(phoneNumber, jid) // keep the cache warm for teardown events

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
		h.asyncDeliverWebhook(traceID, phoneNumber, jid, v)

	case *events.LoggedOut:
		// Capture the jid BEFORE eviction/FK-cascade so the webhook can still
		// address the account. Record status first (Phase D), then evict, then
		// emit session.logged_out.
		jid := sessionJID(phoneNumber)
		h.recordSessionStatus(phoneNumber, "logged_out", loggedOutReason(v), nil)
		clients.Delete(phoneNumber)
		h.dispatch(traceID, phoneNumber, jid, string(domainQueue.EventSessionLoggedOut), loggedOutPayload(phoneNumber, jid, v))

	case *events.TemporaryBan:
		jid := sessionJID(phoneNumber)
		var exp *time.Time
		if v.Expire > 0 {
			t := time.Now().Add(v.Expire)
			exp = &t
		}
		h.recordSessionStatus(phoneNumber, "banned", v.String(), exp)
		clients.Delete(phoneNumber)
		h.dispatch(traceID, phoneNumber, jid, string(domainQueue.EventSessionBanned), bannedPayload(phoneNumber, jid, v))

	case *events.ConnectFailure:
		jid := sessionJID(phoneNumber)
		if v.Reason.IsLoggedOut() {
			h.recordSessionStatus(phoneNumber, "logged_out", v.Reason.String(), nil)
			clients.Delete(phoneNumber)
		} else {
			// Transient/other failure: record but do NOT evict so whatsmeow's
			// auto-reconnect can recover the same client object.
			h.recordSessionStatus(phoneNumber, "disconnected", "connect_failure:"+v.Reason.String(), nil)
		}
		h.dispatch(traceID, phoneNumber, jid, string(domainQueue.EventSessionConnectFailure), connectFailurePayload(phoneNumber, jid, v))

	case *events.StreamReplaced:
		// Another process took the socket; our client is dead. Evict; do not
		// disconnect (already gone).
		jid := sessionJID(phoneNumber)
		h.recordSessionStatus(phoneNumber, "disconnected", "stream_replaced", nil)
		clients.Delete(phoneNumber)
		h.dispatch(traceID, phoneNumber, jid, string(domainQueue.EventSessionReplaced), baseSessionPayload(string(domainQueue.EventSessionReplaced), phoneNumber, jid))

	case *events.NotifyAccountReachoutTimelock:
		// WhatsApp restricted this account from starting conversations with
		// people it has no history with — the enforcement behind ack code 463.
		// The socket and session stay healthy, so record and notify but never
		// evict the client. deriveItem keeps reporting a live client as
		// "connected"; this status only surfaces once the client is gone.
		jid := sessionJID(phoneNumber)
		if v.IsActive {
			var exp *time.Time
			if !v.TimeEnforcementEnds.IsZero() {
				t := v.TimeEnforcementEnds.Time
				exp = &t
			}
			h.recordSessionStatus(phoneNumber, "reachout_timelocked", reachoutTimelockReason(v), exp)
		} else {
			h.recordSessionStatus(phoneNumber, "connected", "reachout_timelock_cleared", nil)
		}
		h.dispatch(traceID, phoneNumber, jid, string(domainQueue.EventSessionReachoutTimelocked), reachoutTimelockPayload(phoneNumber, jid, v))

	case *events.Connected:
		jid := sessionJID(phoneNumber)
		h.dispatch(traceID, phoneNumber, jid, string(domainQueue.EventSessionConnected), baseSessionPayload(string(domainQueue.EventSessionConnected), phoneNumber, jid))

	case *events.Disconnected:
		jid := sessionJID(phoneNumber)
		h.dispatch(traceID, phoneNumber, jid, string(domainQueue.EventSessionDisconnected), baseSessionPayload(string(domainQueue.EventSessionDisconnected), phoneNumber, jid))
	}
}

// baseSessionPayload builds the flat envelope common to every session.* event.
func baseSessionPayload(event, phoneNumber, jid string) map[string]interface{} {
	return map[string]interface{}{
		"event":        event,
		"phone_number": phoneNumber,
		"jid":          jid,
		"timestamp":    time.Now().Unix(),
	}
}

// loggedOutPayload maps *events.LoggedOut to its webhook payload. reason is only
// meaningful when on_connect is true (whatsmeow sets it on connect failures).
func loggedOutPayload(phoneNumber, jid string, v *events.LoggedOut) map[string]interface{} {
	p := baseSessionPayload(string(domainQueue.EventSessionLoggedOut), phoneNumber, jid)
	p["on_connect"] = v.OnConnect
	p["reason"] = int(v.Reason)
	p["reason_text"] = v.Reason.String()
	return p
}

// bannedPayload maps *events.TemporaryBan to its webhook payload.
func bannedPayload(phoneNumber, jid string, v *events.TemporaryBan) map[string]interface{} {
	p := baseSessionPayload(string(domainQueue.EventSessionBanned), phoneNumber, jid)
	p["code"] = int(v.Code)
	p["reason_text"] = v.Code.String()
	p["expires_in"] = int(v.Expire.Seconds())
	return p
}

// reachoutTimelockReason renders the persisted status reason for a reach-out
// timelock, keeping the enforcement type when WhatsApp supplied one.
func reachoutTimelockReason(v *events.NotifyAccountReachoutTimelock) string {
	if v.EnforcementType == "" {
		return "reachout_timelock"
	}
	return "reachout_timelock:" + v.EnforcementType
}

// reachoutTimelockPayload maps *events.NotifyAccountReachoutTimelock to its
// webhook payload. time_enforcement_ends is omitted when WhatsApp did not say
// when the lock lifts.
func reachoutTimelockPayload(phoneNumber, jid string, v *events.NotifyAccountReachoutTimelock) map[string]interface{} {
	p := baseSessionPayload(string(domainQueue.EventSessionReachoutTimelocked), phoneNumber, jid)
	p["is_active"] = v.IsActive
	if v.EnforcementType != "" {
		p["enforcement_type"] = v.EnforcementType
	}
	if !v.TimeEnforcementEnds.IsZero() {
		p["time_enforcement_ends"] = v.TimeEnforcementEnds.Unix()
	}
	return p
}

// connectFailurePayload maps *events.ConnectFailure to its webhook payload.
func connectFailurePayload(phoneNumber, jid string, v *events.ConnectFailure) map[string]interface{} {
	p := baseSessionPayload(string(domainQueue.EventSessionConnectFailure), phoneNumber, jid)
	p["reason"] = int(v.Reason)
	p["reason_text"] = v.Reason.String()
	p["message"] = v.Message
	return p
}

// sessionJID resolves an account's device JID. It reads the live client's
// Store.ID exactly once (avoiding a TOCTOU nil-deref while whatsmeow nils it out
// during logout) and caches the result; when the live client is mid-teardown
// (Store.ID already nil) it falls back to the last-known cached JID so a
// logout/ban webhook can still be addressed. "" only when never paired.
func sessionJID(phoneNumber string) string {
	client := clients.Get(phoneNumber)
	if client != nil && client.Store != nil {
		if id := client.Store.ID; id != nil { // single read: no check-then-deref race
			jid := id.String()
			clients.SetJID(phoneNumber, jid)
			return jid
		}
	}
	return clients.JID(phoneNumber)
}

// dispatch fans a pre-built lifecycle event out to every matching subscription.
// It is non-blocking (queue-publish, else bounded async send) so it is safe to
// call from the whatsmeow event goroutine. A missing jid is a silent no-op.
func (h *handler) dispatch(traceID, phoneNumber, jid, event string, payload map[string]interface{}) {
	if jid == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	subs, err := h.repository.GetWebhookSubscriptions(ctx, jid)
	if err != nil {
		h.logger.Error(traceID, "Failed to load webhook subscriptions for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return
	}

	for _, sub := range subs {
		if !domainQueue.EventMatches(sub.Events, event) {
			continue
		}
		if h.queue != nil && h.queue.IsHealthy() {
			if perr := h.queue.PublishWebhookDelivery(context.Background(), domainQueue.WebhookDeliveryMessage{
				TraceID:    traceID,
				WebhookURL: sub.Url,
				HmacSecret: sub.HmacSecret,
				Payload:    payload,
				// traceID is unique per lifecycle event; combined with the
				// per-destination dedup key it keeps each subscription's
				// delivery independent (no cross-sub collapse) while still
				// deduping a genuine RabbitMQ redelivery of the same message.
				MessageID: traceID,
			}); perr == nil {
				continue
			}
			// Publish failed: fall through to direct async delivery.
		}
		h.sender.SendAsync(sub.Url, sub.HmacSecret, event, payload)
	}
}

// loggedOutReason describes why a LoggedOut event fired: the connect-failure
// reason when triggered on connect, else a stream error.
func loggedOutReason(v *events.LoggedOut) string {
	if v.OnConnect {
		return v.Reason.String()
	}
	return "stream_error"
}

// recordSessionStatus persists a lifecycle state on its own bounded context.
// Failures are logged, never propagated: whatsmeow event handlers cannot return
// errors, and the caller still evicts the client so it stops looking alive.
func (h *handler) recordSessionStatus(phoneNumber, state, reason string, banExpiresAt *time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.repository.UpsertSessionStatus(ctx, phoneNumber, state, reason, banExpiresAt); err != nil {
		h.logger.Error("", "Failed to persist session status for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
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

	// Load every subscription for this account and fan the incoming-message
	// event out to each matching one. A subscription with an empty filter
	// receives everything (the legacy single-URL behavior).
	subs, err := h.repository.GetWebhookSubscriptions(ctx, jid)
	if err != nil || len(subs) == 0 {
		return
	}

	// Build the payload once (media is downloaded once, then reused per sub).
	client := clients.Get(phoneNumber)
	payload := buildWebhookPayload(msg, h.mediaDownloader, traceID, phoneNumber, client)

	const event = string(domainQueue.EventMessageIncoming)
	for _, sub := range subs {
		if !domainQueue.EventMatches(sub.Events, event) {
			continue
		}
		// Validate the URL to prevent SSRF; a bad URL skips only that sub.
		if err := utils.ValidateURL(sub.Url); err != nil {
			h.logger.Error(traceID, "Invalid webhook URL format, skipping delivery", nil, customLog.Error(err))
			continue
		}
		err := h.sender.Send(ctx, sub.Url, sub.HmacSecret, payload)
		metrics.RecordWebhook("direct", event, err)
		if err != nil {
			h.logger.Error(traceID, "Failed to deliver webhook for "+MaskedPhoneNumber(phoneNumber)+" to "+sub.Url, nil, customLog.Error(err))
		} else {
			h.logger.Info(traceID, "Successfully delivered webhook for "+MaskedPhoneNumber(phoneNumber)+" to "+sub.Url, nil)
		}
	}
}

func buildWebhookPayload(msg *events.Message, mediaDownloader MediaDownloader, traceID string, phoneNumber string, client *whatsmeow.Client) map[string]interface{} {
	from, addrMode := ConvertJIDToNonADLID(msg.Info.Sender, msg.Info.SenderAlt, msg.Info.Chat, client)
	payload := map[string]interface{}{
		"event":           string(domainQueue.EventMessageIncoming),
		"message_id":      msg.Info.ID,
		"timestamp":       msg.Info.Timestamp.Unix(),
		"from":            from,
		"addressing_mode": addrMode,
		"chat":            msg.Info.Chat.String(),
		"is_group":        msg.Info.IsGroup,
		"push_name":       msg.Info.PushName,
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
		if name := msg.Message.LocationMessage.GetName(); name != "" {
			payload["name"] = name
		}
		if addr := msg.Message.LocationMessage.GetAddress(); addr != "" {
			payload["address"] = addr
		}

	case msg.Message.PollCreationMessage != nil:
		payload["type"] = "poll"
		payload["question"] = msg.Message.PollCreationMessage.GetName()
		var opts []string
		for _, o := range msg.Message.PollCreationMessage.GetOptions() {
			opts = append(opts, o.GetOptionName())
		}
		payload["options"] = opts
		payload["selectable_count"] = msg.Message.PollCreationMessage.GetSelectableOptionsCount()

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
