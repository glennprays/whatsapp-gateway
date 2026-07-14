package whatsapp

// ContactCheckRequest represents a request to validate a recipient. Accepts
// `chat` (canonical) or the legacy `msisdn` alias; at least one is required.
type ContactCheckRequest struct {
	Chat   string `json:"chat" query:"chat" form:"chat" binding:"omitempty"`
	Msisdn string `json:"msisdn" query:"msisdn" form:"msisdn" binding:"omitempty"` // deprecated: alias for chat
}

// ContactCheckResponse is the result of an IsOnWhatsApp lookup.
type ContactCheckResponse struct {
	Query        string  `json:"query"`         // the phone number queried
	JID          string  `json:"jid"`           // canonical WhatsApp JID ("@s.whatsapp.net")
	IsOnWhatsApp bool    `json:"is_on_whatsapp"` // whether the number is registered
	VerifiedName *string `json:"verified_name,omitempty"` // business verified name, if any
}
