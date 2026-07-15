package whatsapp

// MarkReadRequest marks one or more messages in a chat as read (blue ticks).
// For a group chat, Sender (the message author's JID/number) is required so the
// receipt is attributed correctly.
type MarkReadRequest struct {
	Chat       string   `json:"chat" binding:"omitempty"`
	Msisdn     string   `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	MessageIDs []string `json:"message_ids"`
	Sender     string   `json:"sender" binding:"omitempty"` // required for group chats
}

// ChatPresenceRequest sets the typing indicator in a chat. State is one of
// "composing" (typing…), "recording" (recording a voice note), or "paused"
// (cleared). "recording" is sent as composing + audio media.
type ChatPresenceRequest struct {
	Chat   string `json:"chat" binding:"omitempty"`
	Msisdn string `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	State  string `json:"state"`
}
