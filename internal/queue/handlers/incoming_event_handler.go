package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	customLog "github.com/glennprays/log"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/queue"
)

type IncomingEventHandler struct {
	Repository      whatsapp.WhatsAppRepository
	Publisher       *queue.RabbitMQQueue
	Logger          *customLog.Logger
	MediaDownloader whatsapp.MediaDownloader
	MediaTimeout    time.Duration
	ClientStore     *whatsapp.ClientStore
	Dedup           *queue.DedupCache
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

	// Skip messages already processed within the dedup TTL (the queue is
	// at-least-once, so redeliveries after crashes/requeues are expected)
	if h.Dedup.IsDuplicate(queue.QueueIncomingEvents, eventMsg.MessageID) {
		h.Logger.Info(traceID, fmt.Sprintf("Skipping duplicate incoming event %s", eventMsg.MessageID), nil)
		return nil
	}

	// Get client for JID resolution
	var client *whatsmeow.Client
	if h.ClientStore != nil {
		client = h.ClientStore.Get(eventMsg.PhoneNumber)
	}

	// Build webhook payload with client
	payload := whatsapp.BuildWebhookPayload(&waEvent, h.MediaDownloader, traceID, eventMsg.PhoneNumber, client, h.MediaTimeout)

	// Fan the incoming-message event out to every matching subscription.
	subs, err := h.Repository.GetWebhookSubscriptions(ctx, eventMsg.JID)
	if err != nil {
		return fmt.Errorf("failed to get webhook subscriptions: %w", err)
	}
	if len(subs) == 0 {
		h.Logger.Debug("", fmt.Sprintf("No webhook subscriptions configured for phone %s", eventMsg.PhoneNumber), nil)
		h.Dedup.MarkProcessed(queue.QueueIncomingEvents, eventMsg.MessageID)
		return nil // Not an error, just skip
	}

	event := string(domainQueue.EventMessageIncoming)
	for _, sub := range subs {
		if !domainQueue.EventMatches(sub.Events, event) {
			continue
		}
		webhookMsg := domainQueue.WebhookDeliveryMessage{
			WebhookURL: sub.Url,
			HmacSecret: sub.HmacSecret,
			Payload:    payload,
			MessageID:  eventMsg.MessageID,
		}
		if err := h.Publisher.PublishWebhookDelivery(ctx, webhookMsg); err != nil {
			return fmt.Errorf("failed to publish webhook delivery: %w", err)
		}
	}

	h.Dedup.MarkProcessed(queue.QueueIncomingEvents, eventMsg.MessageID)
	h.Logger.Debug(traceID, fmt.Sprintf("Queued webhook delivery for message %s", eventMsg.MessageID), nil)
	return nil
}
