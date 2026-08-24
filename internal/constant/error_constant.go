package constant

const (
	ErrClientNotFound        = "no active WhatsApp session for this number; link via QR or pair code"
	ErrPhoneNumberNotFound   = "phone number not found in context"
	ErrClientAlreadyLoggedIn = "client already logged in"
	ErrClientNotLoggedIn     = "client not logged in"
	ErrClientSessionDeleted  = "WhatsApp session was deleted by server, please re-pair via QR or pair code"
	ErrWhatsappTimeout       = "whatsapp did not respond in time; the operation was aborted before completion"
)
