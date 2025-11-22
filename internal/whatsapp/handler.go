package whatsapp

type (
	handler struct{}
	Handler interface {
		HandleEvent(jid string, evt any)
	}
)

func NewHandler() Handler {
	return &handler{}
}

// Handle messages, events, QR
func (h *handler) HandleEvent(jid string, evt any) {
}
