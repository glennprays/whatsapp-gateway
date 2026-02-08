package queue

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
