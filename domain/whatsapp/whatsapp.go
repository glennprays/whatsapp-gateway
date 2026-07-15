package whatsapp

// Webhook is the register/upsert request for one subscription. Events is an
// optional subset of the event catalog; omitted or empty means all events.
type Webhook struct {
	Url        string   `json:"url" binding:"required,url"`
	HmacSecret string   `json:"hmac_secret" binding:"omitempty"`
	Events     []string `json:"events" binding:"omitempty"`
}

// WebhookSubscription is one stored subscription. Events is the comma-separated
// catalog subset (” = all events). HmacSecret is encrypted at rest and is
// never serialized back to clients.
type WebhookSubscription struct {
	Url        string
	HmacSecret string
	Events     string
}
