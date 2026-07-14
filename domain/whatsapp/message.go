package whatsapp

// Recipient addressing: every send/message request accepts `chat` (the canonical
// recipient — a bare msisdn, a PN JID "@s.whatsapp.net", a group JID "@g.us", or
// a "@lid") and `msisdn` (a permanent back-compat alias resolving to a PN JID).
// When both are set, `chat` wins. At least one is required (enforced in
// resolveChat, not via struct binding, so `chat`-only requests validate).

// MessageContext carries the optional reply + mentions metadata attached to a
// send. It is caller-supplied (storeless per ADR/decision #5): a reply quotes an
// existing message by id + author, with the quoted preview text supplied by the
// caller. Mentions are the recipients to @-tag. All fields resolved to canonical
// JIDs by the usecase before reaching the client.
type MessageContext struct {
	ReplyToID     string   // quoted message id (StanzaID)
	ReplyToSender string   // quoted message author JID (Participant)
	ReplyToText   string   // caller-supplied quoted preview text (optional)
	Mentions      []string // JIDs to @-mention
}

// IsEmpty reports whether there is no reply and no mentions to attach.
func (m *MessageContext) IsEmpty() bool {
	return m == nil || (m.ReplyToID == "" && len(m.Mentions) == 0)
}

// SendTextMessageRequest represents a text message send request
type SendTextMessageRequest struct {
	Chat          string   `json:"chat" binding:"omitempty"`
	Msisdn        string   `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	Message       string   `json:"message" binding:"required"`
	ReplyToID     string   `json:"reply_to_id,omitempty"`     // quote this message id
	ReplyToSender string   `json:"reply_to_sender,omitempty"` // author of the quoted message (number/JID)
	ReplyToText   string   `json:"reply_to_text,omitempty"`   // optional quoted preview text
	Mentions      []string `json:"mentions,omitempty"`        // numbers/JIDs to @-mention
}

// SendImageMessageRequest represents an image message send request
type SendImageMessageRequest struct {
	Chat          string   `form:"chat" binding:"omitempty"`
	Msisdn        string   `form:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	Caption       string   `form:"caption" binding:"omitempty"`
	IsViewOnce    bool     `form:"is_view_once" binding:"omitempty"`
	ReplyToID     string   `form:"reply_to_id" binding:"omitempty"`     // quote this message id
	ReplyToSender string   `form:"reply_to_sender" binding:"omitempty"` // author of the quoted message (number/JID)
	ReplyToText   string   `form:"reply_to_text" binding:"omitempty"`   // optional quoted preview text
	Mentions      []string `form:"mentions" binding:"omitempty"`        // numbers/JIDs to @-mention (repeated form field)
}

// MessageReactionRequest represents a message reaction request
type MessageReactionRequest struct {
	Chat      string `json:"chat" binding:"omitempty"`
	Msisdn    string `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	MessageID string `json:"message_id" binding:"required"`
	Emoji     string `json:"emoji" binding:"required"`
	// SenderMsisdn is the JID/phone of the original message's sender. It accepts
	// the same forms as chat (bare msisdn or a PN/@lid JID). Omit it when
	// reacting to your own outgoing message (FromMe=true). Required for correct
	// attribution in groups and for reacting to incoming DM messages.
	SenderMsisdn string `json:"sender_msisdn,omitempty"`
}

// MessageDeleteRequest represents a message deletion request
type MessageDeleteRequest struct {
	Chat      string `json:"chat" binding:"omitempty"`
	Msisdn    string `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	MessageID string `json:"message_id" binding:"required"`
}

// MessageEditRequest represents a message edit request
type MessageEditRequest struct {
	Chat       string `json:"chat" binding:"omitempty"`
	Msisdn     string `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	MessageID  string `json:"message_id" binding:"required"`
	NewMessage string `json:"new_message" binding:"required"`
}

// SendLocationMessageRequest represents a location message send request
type SendLocationMessageRequest struct {
	Chat      string  `json:"chat" binding:"omitempty"`
	Msisdn    string  `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// SendPollMessageRequest represents a poll message send request
type SendPollMessageRequest struct {
	Chat            string   `json:"chat" binding:"omitempty"`
	Msisdn          string   `json:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	Question        string   `json:"question" binding:"required"`
	Options         []string `json:"options" binding:"required"`
	SelectableCount int      `json:"selectable_count,omitempty"`
}

// SendStickerMessageRequest represents a sticker message send request
type SendStickerMessageRequest struct {
	Chat   string `form:"chat" binding:"omitempty"`
	Msisdn string `form:"msisdn" binding:"omitempty"` // deprecated: alias for chat
}

// SendAudioMessageRequest represents an audio message send request.
// IsPTT=true renders the waveform "voice note" bubble; false renders a
// playable audio-file card.
type SendAudioMessageRequest struct {
	Chat          string   `form:"chat" binding:"omitempty"`
	Msisdn        string   `form:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	IsViewOnce    bool     `form:"is_view_once" binding:"omitempty"`
	IsPTT         bool     `form:"is_ptt" binding:"omitempty"`
	ReplyToID     string   `form:"reply_to_id" binding:"omitempty"`     // quote this message id
	ReplyToSender string   `form:"reply_to_sender" binding:"omitempty"` // author of the quoted message (number/JID)
	ReplyToText   string   `form:"reply_to_text" binding:"omitempty"`   // optional quoted preview text
	Mentions      []string `form:"mentions" binding:"omitempty"`        // numbers/JIDs to @-mention (repeated form field)
}

// SendVideoMessageRequest represents a video message send request.
type SendVideoMessageRequest struct {
	Chat          string   `form:"chat" binding:"omitempty"`
	Msisdn        string   `form:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	Caption       string   `form:"caption" binding:"omitempty"`
	IsViewOnce    bool     `form:"is_view_once" binding:"omitempty"`
	IsGif         bool     `form:"is_gif" binding:"omitempty"`
	ReplyToID     string   `form:"reply_to_id" binding:"omitempty"`     // quote this message id
	ReplyToSender string   `form:"reply_to_sender" binding:"omitempty"` // author of the quoted message (number/JID)
	ReplyToText   string   `form:"reply_to_text" binding:"omitempty"`   // optional quoted preview text
	Mentions      []string `form:"mentions" binding:"omitempty"`        // numbers/JIDs to @-mention (repeated form field)
}

// SendDocumentMessageRequest represents a document (file) message send request.
type SendDocumentMessageRequest struct {
	Chat     string `form:"chat" binding:"omitempty"`
	Msisdn   string `form:"msisdn" binding:"omitempty"` // deprecated: alias for chat
	Caption  string `form:"caption" binding:"omitempty"`
	FileName string `form:"file_name" binding:"omitempty"`
}

// SendMessageResponse represents a successful message send response
type SendMessageResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id"`
	Chat      string `json:"chat"` // resolved canonical recipient JID
}

// MessageOperationResponse represents a successful message operation response
type MessageOperationResponse struct {
	Success bool `json:"success"`
}

// SendMessageQueuedResponse represents a queued message response
type SendMessageQueuedResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"` // "queued"
	JobID   string `json:"job_id"`
	Chat    string `json:"chat"` // resolved canonical recipient JID
}

// JobStatusResponse represents a job status lookup response
type JobStatusResponse struct {
	JobID       string  `json:"job_id"`
	Status      string  `json:"status"` // "queued", "processing", "completed", "failed"
	MessageID   *string `json:"message_id,omitempty"`
	Error       *string `json:"error,omitempty"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}
