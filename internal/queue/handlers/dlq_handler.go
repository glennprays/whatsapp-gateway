package handlers

import (
	"context"
	"encoding/json"
	"time"

	customLog "github.com/glennprays/log"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	pkgQueue "github.com/glennprays/whatsapp-gateway/pkg/queue"
)

// DLQHandler consumes dead-lettered messages from all DLQs so failures that
// exhausted their retries are observable instead of silently piling up. It
// logs full context and, where a webhook is configured, notifies the client
// with a message.failed payload marked as dead-lettered.
type DLQHandler struct {
	Manager    whatsapp.Manager
	Repository whatsapp.WhatsAppRepository
	Sender     *whatsapp.WebhookSender
	Logger     *customLog.Logger
	Config     *config.Config
}

// Handle always returns nil: dead-lettered messages must never re-enter the
// retry flow, and the DLQs have no dead-letter target of their own.
func (h *DLQHandler) Handle(ctx context.Context, body []byte, headers amqp.Table) error {
	traceID := pkgQueue.GetTraceIDWorkerProcess(headers)
	sourceQueue := dlqSourceQueue(headers)

	h.Logger.Error(traceID, "Message dead-lettered after exhausting retries", nil,
		customLog.String("source_queue", sourceQueue),
		customLog.String("body", string(body)),
	)

	switch sourceQueue {
	case pkgQueue.QueueOutgoingMessages:
		var job domainQueue.OutgoingMessageJob
		if err := json.Unmarshal(body, &job); err != nil {
			h.Logger.Error(traceID, "Failed to unmarshal dead-lettered outgoing job", nil, customLog.Error(err))
			return nil
		}
		h.notifyDeadLettered(ctx, traceID, job.PhoneNumber, "", map[string]interface{}{
			"event":        string(domainQueue.EventMessageFailed),
			"job_id":       job.JobID,
			"to":           job.To,
			"phone_number": job.PhoneNumber,
			"timestamp":    time.Now().Unix(),
			"error":        "message dead-lettered after exhausting retries",
			"metadata": map[string]interface{}{
				"type":          job.Type,
				"dead_lettered": true,
				"source_queue":  sourceQueue,
			},
		})

	case pkgQueue.QueueIncomingEvents:
		var eventMsg domainQueue.IncomingEventMessage
		if err := json.Unmarshal(body, &eventMsg); err != nil {
			h.Logger.Error(traceID, "Failed to unmarshal dead-lettered incoming event", nil, customLog.Error(err))
			return nil
		}
		h.notifyDeadLettered(ctx, traceID, eventMsg.PhoneNumber, eventMsg.JID, map[string]interface{}{
			"event":        string(domainQueue.EventMessageFailed),
			"phone_number": eventMsg.PhoneNumber,
			"message_id":   eventMsg.MessageID,
			"timestamp":    time.Now().Unix(),
			"error":        "incoming event dead-lettered after exhausting retries",
			"metadata": map[string]interface{}{
				"dead_lettered": true,
				"source_queue":  sourceQueue,
			},
		})

	case pkgQueue.QueueWebhookDelivery:
		// The webhook endpoint itself is failing — notifying it again is
		// futile, so log with enough context for manual follow-up.
		var webhookMsg domainQueue.WebhookDeliveryMessage
		if err := json.Unmarshal(body, &webhookMsg); err != nil {
			h.Logger.Error(traceID, "Failed to unmarshal dead-lettered webhook delivery", nil, customLog.Error(err))
			return nil
		}
		h.Logger.Error(traceID, "Webhook delivery dead-lettered; receiver kept failing", nil,
			customLog.String("webhook_url", webhookMsg.WebhookURL),
			customLog.String("message_id", webhookMsg.MessageID),
		)

	default:
		h.Logger.Error(traceID, "Dead-lettered message from unknown source queue", nil,
			customLog.String("source_queue", sourceQueue),
		)
	}

	return nil
}

// notifyDeadLettered sends a message.failed webhook (marked dead_lettered) to
// the client's configured webhook, honoring the status-webhook settings.
// jid may be empty, in which case it is resolved from the phone number.
func (h *DLQHandler) notifyDeadLettered(ctx context.Context, traceID, phoneNumber, jid string, payload map[string]interface{}) {
	if !h.Config.WebhookStatusEventsEnabled {
		return
	}

	masked := whatsapp.MaskedPhoneNumber(phoneNumber)

	if jid == "" {
		resolved, err := h.Manager.GetJIDFromPhoneNumber(phoneNumber)
		if err != nil {
			h.Logger.Error(traceID, "Failed to resolve JID for dead-letter notification", nil,
				customLog.String("phone_number", masked),
				customLog.Error(err),
			)
			return
		}
		jid = resolved
	}

	subs, err := h.Repository.GetWebhookSubscriptions(ctx, jid)
	if err != nil {
		h.Logger.Error(traceID, "Failed to get webhook subscriptions for dead-letter notification", nil,
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return
	}
	if len(subs) == 0 {
		h.Logger.Debug(traceID, "No webhook subscriptions configured for dead-letter notification", nil,
			customLog.String("phone_number", masked),
		)
		return
	}

	// The dead-letter payload is a message.failed event; deliver to each
	// subscription whose events filter matches (empty = all).
	event := string(domainQueue.EventMessageFailed)
	for _, sub := range subs {
		if !domainQueue.EventMatches(sub.Events, event) {
			continue
		}
		if err := h.Sender.Send(ctx, sub.Url, sub.HmacSecret, payload); err != nil {
			h.Logger.Error(traceID, "Failed to send dead-letter notification webhook", nil,
				customLog.String("phone_number", masked),
				customLog.Error(err),
			)
			continue
		}
		h.Logger.Info(traceID, "Sent dead-letter notification webhook", nil,
			customLog.String("phone_number", masked),
		)
	}
}

// dlqSourceQueue extracts the queue a message was dead-lettered from using
// the x-death headers RabbitMQ attaches. The "rejected" death identifies the
// main queue (retry-queue deaths have reason "expired").
func dlqSourceQueue(headers amqp.Table) string {
	deaths, ok := headers["x-death"].([]interface{})
	if !ok {
		return ""
	}

	for _, d := range deaths {
		death, ok := d.(amqp.Table)
		if !ok {
			continue
		}
		if reason, _ := death["reason"].(string); reason != "rejected" {
			continue
		}
		if queueName, ok := death["queue"].(string); ok {
			return queueName
		}
	}

	// Fallback: first death entry.
	if len(deaths) > 0 {
		if death, ok := deaths[0].(amqp.Table); ok {
			if queueName, ok := death["queue"].(string); ok {
				return queueName
			}
		}
	}
	return ""
}
