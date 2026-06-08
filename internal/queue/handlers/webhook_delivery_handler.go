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
	// after crashes/requeues would otherwise hit the receiver twice)
	if h.Dedup.IsDuplicate(queue.QueueWebhookDelivery, webhookMsg.MessageID) {
		h.Logger.Info(traceID, fmt.Sprintf("Skipping duplicate webhook delivery for message %s", webhookMsg.MessageID), nil)
		return nil
	}

	// Deliver webhook
	if err := h.Sender.Send(ctx, webhookMsg.WebhookURL, webhookMsg.HmacSecret, webhookMsg.Payload); err != nil {
		return fmt.Errorf("failed to deliver webhook: %w", err)
	}

	h.Dedup.MarkProcessed(queue.QueueWebhookDelivery, webhookMsg.MessageID)
	h.Logger.Debug(traceID, fmt.Sprintf("Successfully delivered webhook for message %s", webhookMsg.MessageID), nil)
	return nil
}
