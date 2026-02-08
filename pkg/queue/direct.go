package queue

import (
	"context"

	customLog "github.com/glennprays/log"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
)

// DirectQueue is a no-op implementation (fallback when RabbitMQ disabled)
type DirectQueue struct {
	logger *customLog.Logger
}

func NewDirectQueue(logger *customLog.Logger) domainQueue.MessageQueue {
	return &DirectQueue{logger: logger}
}

// All Publish methods return nil (direct processing happens elsewhere)
func (d *DirectQueue) PublishIncomingEvent(ctx context.Context, event domainQueue.IncomingEventMessage) error {
	return nil
}

func (d *DirectQueue) PublishOutgoingMessage(ctx context.Context, job domainQueue.OutgoingMessageJob) error {
	return nil
}

func (d *DirectQueue) PublishWebhookDelivery(ctx context.Context, msg domainQueue.WebhookDeliveryMessage) error {
	return nil
}

func (d *DirectQueue) IsHealthy() bool {
	return true
}
