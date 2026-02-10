package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	customLog "github.com/glennprays/log"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mau.fi/whatsmeow/types/events"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/queue"
)

type IncomingEventHandler struct {
	Repository whatsapp.WhatsAppRepository
	Publisher  *queue.RabbitMQQueue
	Logger     *customLog.Logger
}

func (h *IncomingEventHandler) Handle(ctx context.Context, body []byte, headers amqp.Table) error {
	traceID := uuid.New().String()
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

	// Build webhook payload
	payload := buildWebhookPayload(&waEvent)

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

func buildWebhookPayload(msg *events.Message) map[string]interface{} {
	payload := map[string]interface{}{
		"message_id": msg.Info.ID,
		"timestamp":  msg.Info.Timestamp.Unix(),
		"from":       msg.Info.Sender.String(),
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
		mediaInfo = map[string]interface{}{
			"type":      "image",
			"url":       msg.Message.ImageMessage.GetURL(),
			"mime_type": msg.Message.ImageMessage.GetMimetype(),
			"size":      msg.Message.ImageMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.ImageMessage.GetFileSHA256()),
		}
		if caption := msg.Message.ImageMessage.GetCaption(); caption != "" {
			mediaInfo["caption"] = caption
		}

	case msg.Message.VideoMessage != nil:
		payload["type"] = "video"
		mediaInfo = map[string]interface{}{
			"type":      "video",
			"url":       msg.Message.VideoMessage.GetURL(),
			"mime_type": msg.Message.VideoMessage.GetMimetype(),
			"size":      msg.Message.VideoMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.VideoMessage.GetFileSHA256()),
		}
		if caption := msg.Message.VideoMessage.GetCaption(); caption != "" {
			mediaInfo["caption"] = caption
		}

	case msg.Message.AudioMessage != nil:
		payload["type"] = "audio"
		mediaInfo = map[string]interface{}{
			"type":      "audio",
			"url":       msg.Message.AudioMessage.GetURL(),
			"mime_type": msg.Message.AudioMessage.GetMimetype(),
			"size":      msg.Message.AudioMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.AudioMessage.GetFileSHA256()),
		}

	case msg.Message.DocumentMessage != nil:
		payload["type"] = "document"
		mediaInfo = map[string]interface{}{
			"type":      "document",
			"url":       msg.Message.DocumentMessage.GetURL(),
			"file_name": msg.Message.DocumentMessage.GetFileName(),
			"mime_type": msg.Message.DocumentMessage.GetMimetype(),
			"size":      msg.Message.DocumentMessage.GetFileLength(),
			"sha256":    fmt.Sprintf("%x", msg.Message.DocumentMessage.GetFileSHA256()),
		}

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
