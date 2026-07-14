package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	customLog "github.com/glennprays/log"
	amqp "github.com/rabbitmq/amqp091-go"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/queue"
)

type WebhookDeliveryHandler struct {
	Sender *whatsapp.WebhookSender
	Logger *customLog.Logger
	Dedup  *queue.DedupCache
}

func (h *WebhookDeliveryHandler) Handle(ctx context.Context, body []byte, headers amqp.Table) error {
	traceID := queue.GetTraceIDWorkerProcess(headers)

	// Unmarshal webhook delivery message
	var webhookMsg domainQueue.WebhookDeliveryMessage
	if err := json.Unmarshal(body, &webhookMsg); err != nil {
		return fmt.Errorf("failed to unmarshal webhook message: %w", err)
	}

	// Skip webhooks already delivered within the dedup TTL (redeliveries
	// after crashes/requeues would otherwise hit the receiver twice). The key
	// is scoped per destination URL: one event fanned out to N subscriptions
	// produces N messages sharing a MessageID, and a MessageID-only key would
	// collapse them to a single delivery (dropping every URL after the first).
	dedupKey := webhookDedupKey(webhookMsg.MessageID, webhookMsg.WebhookURL)
	if h.Dedup.IsDuplicate(queue.QueueWebhookDelivery, dedupKey) {
		h.Logger.Info(traceID, fmt.Sprintf("Skipping duplicate webhook delivery for %s", dedupKey), nil)
		return nil
	}

	// Deliver webhook
	if err := h.Sender.Send(ctx, webhookMsg.WebhookURL, webhookMsg.HmacSecret, webhookMsg.Payload); err != nil {
		return fmt.Errorf("failed to deliver webhook: %w", err)
	}

	h.Dedup.MarkProcessed(queue.QueueWebhookDelivery, dedupKey)
	h.Logger.Debug(traceID, fmt.Sprintf("Successfully delivered webhook for message %s", webhookMsg.MessageID), nil)
	return nil
}

// webhookDedupKey scopes the delivery dedup to a single (message, destination)
// pair. A fan-out of one event to N subscriptions shares a MessageID, so a
// MessageID-only key would treat sibling deliveries as duplicates and drop
// every URL after the first; appending the URL keeps each destination
// independent while a genuine redelivery of the same message to the same URL
// still dedups (identical key).
func webhookDedupKey(messageID, url string) string {
	return messageID + "|" + url
}
