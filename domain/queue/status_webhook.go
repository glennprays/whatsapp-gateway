package queue

import "strings"

// StatusWebhookEvent represents the type of status event
type StatusWebhookEvent string

const (
	// EventMessageQueued indicates message has been queued for processing
	EventMessageQueued StatusWebhookEvent = "message.queued"
	// EventMessageSent indicates message was successfully sent to WhatsApp
	EventMessageSent StatusWebhookEvent = "message.sent"
	// EventMessageFailed indicates message failed after retries
	EventMessageFailed StatusWebhookEvent = "message.failed"
	// EventMessageIncoming indicates an incoming message was received
	EventMessageIncoming StatusWebhookEvent = "message.incoming"

	// Session lifecycle events (roadmap #1), emitted from the whatsmeow event switch.
	EventSessionLoggedOut      StatusWebhookEvent = "session.logged_out"
	EventSessionBanned         StatusWebhookEvent = "session.banned"
	EventSessionConnectFailure StatusWebhookEvent = "session.connect_failure"
	EventSessionConnected      StatusWebhookEvent = "session.connected"
	EventSessionDisconnected   StatusWebhookEvent = "session.disconnected"
	EventSessionReplaced       StatusWebhookEvent = "session.replaced"
)

// WebhookEventCatalog is the single source of truth for every webhook event a
// subscription may filter on. A subscription's events field is validated
// against this set before it is stored. The session.* entries are appended by
// the generic event-system change (roadmap #1).
var WebhookEventCatalog = map[string]struct{}{
	string(EventMessageIncoming):       {},
	string(EventMessageQueued):         {},
	string(EventMessageSent):           {},
	string(EventMessageFailed):         {},
	string(EventSessionLoggedOut):      {},
	string(EventSessionBanned):         {},
	string(EventSessionConnectFailure): {},
	string(EventSessionConnected):      {},
	string(EventSessionDisconnected):   {},
	string(EventSessionReplaced):       {},
}

// IsKnownEvent reports whether e is a recognized webhook event type.
func IsKnownEvent(e string) bool {
	_, ok := WebhookEventCatalog[e]
	return ok
}

// EventMatches reports whether a subscription with the given comma-separated
// events filter should receive event. An empty filter means "all events"
// (matches the legacy single-URL-gets-everything behavior).
func EventMatches(subEvents, event string) bool {
	if strings.TrimSpace(subEvents) == "" {
		return true
	}
	for _, e := range strings.Split(subEvents, ",") {
		if strings.TrimSpace(e) == event {
			return true
		}
	}
	return false
}

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
