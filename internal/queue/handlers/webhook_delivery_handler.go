package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	customLog "github.com/glennprays/log"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
)

type WebhookDeliveryHandler struct {
	Sender *whatsapp.WebhookSender
	Logger *customLog.Logger
}

func (h *WebhookDeliveryHandler) Handle(ctx context.Context, body []byte, headers amqp.Table) error {
	traceID := uuid.New().String()
	// Unmarshal webhook delivery message
	var webhookMsg domainQueue.WebhookDeliveryMessage
	if err := json.Unmarshal(body, &webhookMsg); err != nil {
		return fmt.Errorf("failed to unmarshal webhook message: %w", err)
	}

	// Deliver webhook
	if err := h.Sender.Send(ctx, webhookMsg.WebhookURL, webhookMsg.HmacSecret, webhookMsg.Payload); err != nil {
		return fmt.Errorf("failed to deliver webhook: %w", err)
	}

	h.Logger.Debug(traceID, fmt.Sprintf("Successfully delivered webhook for message %s", webhookMsg.MessageID), nil)
	return nil
}
