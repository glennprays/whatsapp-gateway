package queue

// StatusWebhookEvent represents the type of status event
type StatusWebhookEvent string

const (
	// EventMessageQueued indicates message has been queued for processing
	EventMessageQueued StatusWebhookEvent = "message.queued"
	// EventMessageSent indicates message was successfully sent to WhatsApp
	EventMessageSent StatusWebhookEvent = "message.sent"
	// EventMessageFailed indicates message failed after retries
	EventMessageFailed StatusWebhookEvent = "message.failed"
)

// StatusWebhookPayload represents the webhook payload for message status updates
type StatusWebhookPayload struct {
	Event       string                 `json:"event"`
	JobID       string                 `json:"job_id"`
	To          string                 `json:"to"`
	PhoneNumber string                 `json:"phone_number"`
	Timestamp   int64                  `json:"timestamp"`
	MessageID   string                 `json:"message_id,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
