package constant

const (
	ErrClientNotFound        = "client not found, please register first"
	ErrPhoneNumberNotFound   = "phone number not found in context"
	ErrClientAlreadyLoggedIn = "client already logged in"
	ErrClientNotLoggedIn     = "client not logged in"
	ErrClientSessionDeleted  = "WhatsApp session was deleted by server, please re-pair via QR or pair code"
)
