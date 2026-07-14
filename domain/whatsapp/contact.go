package whatsapp

// ContactCheckRequest represents a request to validate a recipient. Accepts
// `chat` (canonical) or the legacy `msisdn` alias; at least one is required.
type ContactCheckRequest struct {
	Chat   string `json:"chat" query:"chat" form:"chat" binding:"omitempty"`
	Msisdn string `json:"msisdn" query:"msisdn" form:"msisdn" binding:"omitempty"` // deprecated: alias for chat
}

// ContactCheckResponse is the result of an IsOnWhatsApp lookup.
type ContactCheckResponse struct {
	Query        string  `json:"query"`                   // the phone number queried
	JID          string  `json:"jid"`                     // canonical WhatsApp JID ("@s.whatsapp.net")
	IsOnWhatsApp bool    `json:"is_on_whatsapp"`          // whether the number is registered
	VerifiedName *string `json:"verified_name,omitempty"` // business verified name, if any
}

// ContactListItem is one entry in the account's locally-synced contact list.
type ContactListItem struct {
	JID          string `json:"jid"`
	PushName     string `json:"push_name,omitempty"`
	FullName     string `json:"full_name,omitempty"`
	FirstName    string `json:"first_name,omitempty"`
	BusinessName string `json:"business_name,omitempty"`
}

// ContactListResponse is a page of the account's locally-synced contacts. The
// list reflects whatsmeow's synced address book (populated after pairing and as
// the account is used), so an empty or partial result is not an error.
type ContactListResponse struct {
	Contacts []ContactListItem `json:"contacts"`
	Count    int               `json:"count"` // items in this page
	Total    int               `json:"total"` // total synced contacts
	Note     string            `json:"note"`
}
