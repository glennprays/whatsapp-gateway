package queue

import "context"

// MessageQueue abstracts queue operations (domain layer interface)
type MessageQueue interface {
	// Incoming events
	PublishIncomingEvent(ctx context.Context, event IncomingEventMessage) error

	// Outgoing messages
	PublishOutgoingMessage(ctx context.Context, job OutgoingMessageJob) error

	// Webhook delivery
	PublishWebhookDelivery(ctx context.Context, msg WebhookDeliveryMessage) error

	// Health check
	IsHealthy() bool
}

// IncomingEventMessage represents a WhatsApp incoming event
type IncomingEventMessage struct {
	TraceID     string
	PhoneNumber string
	JID         string
	Event       []byte // JSON-serialized *events.Message
	MessageID   string
	Timestamp   int64
}

// OutgoingMessageJob represents an outgoing message job
type OutgoingMessageJob struct {
	TraceID     string
	JobID       string
	PhoneNumber string
	Type        string // "text", "image", "location", "poll", "sticker", "react", "delete", "edit"
	To          string
	Text        string
	ImageData   string // base64 (image/sticker)
	MimeType    string
	Caption     string
	IsViewOnce  bool
	MessageID   string // For react/delete/edit
	Emoji       string // For react
	NewText     string // For edit

	// Location
	Latitude        float64
	Longitude       float64
	LocationName    string
	LocationAddress string

	// Poll
	Question        string
	Options         []string
	SelectableCount int

	CreatedAt int64
}

// WebhookDeliveryMessage represents a webhook delivery task
type WebhookDeliveryMessage struct {
	TraceID    string
	WebhookURL string
	HmacSecret string // Encrypted
	Payload    map[string]interface{}
	MessageID  string
}
