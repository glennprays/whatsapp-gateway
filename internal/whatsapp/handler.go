package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
)

type (
	handler struct {
		repository WhatsAppRepository
		sender     *WebhookSender
		queue      domainQueue.MessageQueue
	}
	Handler interface {
		HandleEvent(jid string, evt any)
	}
)

func NewHandler(repo WhatsAppRepository, sender *WebhookSender, queue domainQueue.MessageQueue) Handler {
	return &handler{
		repository: repo,
		sender:     sender,
		queue:      queue,
	}
}

// Handle messages, events, QR
func (h *handler) HandleEvent(phoneNumber string, evt any) {
	// Type switch on event
	switch v := evt.(type) {
	case *events.Message:
		// Skip outgoing messages (sent by us)
		if v.Info.IsFromMe {
			return
		}

		// Get client for webhook lookup
		client := Clients[phoneNumber]
		if client == nil || client.Store == nil || client.Store.ID == nil {
			log.Errorf("Client not found for phone %s", MaskedPhoneNumber(phoneNumber))
			return
		}

		jid := client.Store.ID.String()

		// Try to publish to queue if enabled
		if h.queue != nil && h.queue.IsHealthy() {
			eventJSON, err := json.Marshal(v)
			if err != nil {
				log.Errorf("Failed to marshal event for %s: %v", MaskedPhoneNumber(phoneNumber), err)
				// Fallback to direct delivery
				go h.deliverWebhook(phoneNumber, jid, v)
				return
			}

			err = h.queue.PublishIncomingEvent(context.Background(), domainQueue.IncomingEventMessage{
				PhoneNumber: phoneNumber,
				JID:         jid,
				Event:       eventJSON,
				MessageID:   v.Info.ID,
				Timestamp:   v.Info.Timestamp.Unix(),
			})

			if err != nil {
				log.Warnf("Queue publish failed for %s, using direct delivery: %v", MaskedPhoneNumber(phoneNumber), err)
				// Fallback to direct delivery
				go h.deliverWebhook(phoneNumber, jid, v)
				return
			}

			log.Debugf("Successfully queued incoming event for %s", MaskedPhoneNumber(phoneNumber))
			return
		}

		// Direct mode (or fallback): deliver webhook asynchronously
		go h.deliverWebhook(phoneNumber, jid, v)
	}
}

func (h *handler) deliverWebhook(phoneNumber string, jid string, msg *events.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fetch webhook config
	webhook, err := h.repository.GetWebhook(ctx, jid)
	if err != nil || webhook == nil || webhook.Url == "" {
		// No webhook configured, skip silently
		return
	}

	// Build payload
	payload := buildWebhookPayload(msg)

	// Send webhook
	err = h.sender.Send(ctx, webhook.Url, webhook.HmacSecret, payload)
	if err != nil {
		log.Errorf("Failed to deliver webhook for %s to %s: %v", MaskedPhoneNumber(phoneNumber), webhook.Url, err)
	} else {
		log.Infof("Successfully delivered webhook for %s to %s", MaskedPhoneNumber(phoneNumber), webhook.Url)
	}
}

func buildWebhookPayload(msg *events.Message) map[string]interface{} {
	payload := map[string]interface{}{
		"message_id": msg.Info.ID,
		"timestamp":  msg.Info.Timestamp.Unix(),
		"from":       msg.Info.Sender.String(),
		"chat":       msg.Info.Chat.String(),
		"is_group":   msg.Info.IsGroup,
		"push_name":  msg.Info.PushName,
	}

	// Extract message content based on type
	if msg.Message == nil {
		return payload
	}

	switch {
	case msg.Message.Conversation != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.Conversation

	case msg.Message.ExtendedTextMessage != nil:
		payload["type"] = "text"
		payload["text"] = *msg.Message.ExtendedTextMessage.Text

	case msg.Message.ImageMessage != nil:
		payload["type"] = "image"
		mediaInfo := map[string]interface{}{
			"type":      "image",
			"url":       msg.Message.ImageMessage.GetURL(),
			"mime_type": msg.Message.ImageMessage.GetMimetype(),
			"size":      msg.Message.ImageMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.ImageMessage.GetFileSHA256()),
		}
		if caption := msg.Message.ImageMessage.GetCaption(); caption != "" {
			mediaInfo["caption"] = caption
		}
		payload["media"] = mediaInfo

	case msg.Message.VideoMessage != nil:
		payload["type"] = "video"
		mediaInfo := map[string]interface{}{
			"type":      "video",
			"url":       msg.Message.VideoMessage.GetURL(),
			"mime_type": msg.Message.VideoMessage.GetMimetype(),
			"size":      msg.Message.VideoMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.VideoMessage.GetFileSHA256()),
		}
		if caption := msg.Message.VideoMessage.GetCaption(); caption != "" {
			mediaInfo["caption"] = caption
		}
		payload["media"] = mediaInfo

	case msg.Message.AudioMessage != nil:
		payload["type"] = "audio"
		mediaInfo := map[string]interface{}{
			"type":      "audio",
			"url":       msg.Message.AudioMessage.GetURL(),
			"mime_type": msg.Message.AudioMessage.GetMimetype(),
			"size":      msg.Message.AudioMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.AudioMessage.GetFileSHA256()),
		}
		payload["media"] = mediaInfo

	case msg.Message.DocumentMessage != nil:
		payload["type"] = "document"
		mediaInfo := map[string]interface{}{
			"type":      "document",
			"url":       msg.Message.DocumentMessage.GetURL(),
			"file_name": msg.Message.DocumentMessage.GetFileName(),
			"mime_type": msg.Message.DocumentMessage.GetMimetype(),
			"size":      msg.Message.DocumentMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.DocumentMessage.GetFileSHA256()),
		}
		payload["media"] = mediaInfo

	case msg.Message.StickerMessage != nil:
		payload["type"] = "sticker"
		if msg.Message.StickerMessage.Mimetype != nil {
			payload["mime_type"] = *msg.Message.StickerMessage.Mimetype
		}

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

	return payload
}

// GetWebhookURL helper method for compatibility
func (h *handler) GetWebhookURL(ctx context.Context, jid string) (*string, error) {
	webhook, err := h.repository.GetWebhook(ctx, jid)
	if err != nil || webhook == nil {
		return nil, err
	}
	return &webhook.Url, nil
}
