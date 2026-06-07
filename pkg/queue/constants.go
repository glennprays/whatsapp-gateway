package queue

import (
	"fmt"
	"time"
)

const (
	ExchangeName = "whatsapp.topic"
	ExchangeDLX  = "whatsapp.dlx"
	ExchangeType = "topic"

	// Queue names
	QueueIncomingEvents   = "whatsapp.events.incoming"
	QueueWebhookDelivery  = "whatsapp.webhooks.delivery"
	QueueOutgoingMessages = "whatsapp.messages.outgoing"

	// DLQ names
	DLQIncomingEvents   = "whatsapp.dlq.events.incoming"
	DLQWebhookDelivery  = "whatsapp.dlq.webhooks.delivery"
	DLQOutgoingMessages = "whatsapp.dlq.messages.outgoing"

	// Routing keys
	RoutingKeyIncomingEvent = "event.incoming"
	RoutingKeyWebhook       = "webhook.delivery"
	RoutingKeyOutgoingMsg   = "message.outgoing"

)

// RetryTiers are the delay tiers backed by dedicated retry queues with a
// queue-level TTL. Using one queue per delay avoids head-of-line blocking:
// RabbitMQ only dead-letters expired messages from the head of a queue, so
// mixing per-message TTLs in a single queue lets a long delay block shorter
// ones behind it. Delays beyond the largest tier go to the long retry queue,
// which uses per-message expiration and is isolated from the short tiers.
//
// NOTE: this topology replaces the former single "<queue>.retry" queues.
// Existing deployments will see those old queues sit idle; they can be
// deleted via the RabbitMQ management UI once drained.
var RetryTiers = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	25 * time.Second,
}

// retryTierQueueName returns the retry queue name for a delay tier,
// e.g. "whatsapp.messages.outgoing.retry.5s".
func retryTierQueueName(queue string, tier time.Duration) string {
	return fmt.Sprintf("%s.retry.%s", queue, tier)
}

// retryLongQueueName returns the long-delay retry queue name,
// e.g. "whatsapp.messages.outgoing.retry.long".
func retryLongQueueName(queue string) string {
	return queue + ".retry.long"
}

// retryRoute picks the retry queue (and routing key — they are identical)
// for a delay. It returns true when the long queue is used and the delay
// must be carried as a per-message expiration.
func retryRoute(queue string, delay time.Duration) (string, bool) {
	for _, tier := range RetryTiers {
		if delay <= tier {
			return retryTierQueueName(queue, tier), false
		}
	}
	return retryLongQueueName(queue), true
}
