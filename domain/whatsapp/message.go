package whatsapp

// SendTextMessageRequest represents a text message send request
type SendTextMessageRequest struct {
	Msisdn  string `json:"msisdn" binding:"required"`
	Message string `json:"message" binding:"required"`
}

// SendImageMessageRequest represents an image message send request
type SendImageMessageRequest struct {
	Msisdn     string `form:"msisdn" binding:"required"`
	Caption    string `form:"caption" binding:"omitempty"`
	IsViewOnce bool   `form:"is_view_once" binding:"omitempty"`
}

// MessageReactionRequest represents a message reaction request
type MessageReactionRequest struct {
	Msisdn    string `json:"msisdn" binding:"required"`
	MessageID string `json:"message_id" binding:"required"`
	Emoji     string `json:"emoji" binding:"required"`
	// SenderMsisdn is the JID/phone of the original message's sender.
	// Omit it when reacting to your own outgoing message (FromMe=true).
	// Required for correct attribution in groups and for reacting to
	// incoming DM messages.
	SenderMsisdn string `json:"sender_msisdn,omitempty"`
}

// MessageDeleteRequest represents a message deletion request
type MessageDeleteRequest struct {
	Msisdn    string `json:"msisdn" binding:"required"`
	MessageID string `json:"message_id" binding:"required"`
}

// MessageEditRequest represents a message edit request
type MessageEditRequest struct {
	Msisdn     string `json:"msisdn" binding:"required"`
	MessageID  string `json:"message_id" binding:"required"`
	NewMessage string `json:"new_message" binding:"required"`
}

// SendLocationMessageRequest represents a location message send request
type SendLocationMessageRequest struct {
	Msisdn    string  `json:"msisdn" binding:"required"`
	Latitude  float64 `json:"latitude" binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// SendPollMessageRequest represents a poll message send request
type SendPollMessageRequest struct {
	Msisdn          string   `json:"msisdn" binding:"required"`
	Question        string   `json:"question" binding:"required"`
	Options         []string `json:"options" binding:"required"`
	SelectableCount int      `json:"selectable_count,omitempty"`
}

// SendStickerMessageRequest represents a sticker message send request
type SendStickerMessageRequest struct {
	Msisdn string `form:"msisdn" binding:"required"`
}

// SendAudioMessageRequest represents an audio message send request.
// IsPTT=true renders the waveform "voice note" bubble; false renders a
// playable audio-file card.
type SendAudioMessageRequest struct {
	Msisdn     string `form:"msisdn" binding:"required"`
	IsViewOnce bool   `form:"is_view_once" binding:"omitempty"`
	IsPTT      bool   `form:"is_ptt" binding:"omitempty"`
}

// SendMessageResponse represents a successful message send response
type SendMessageResponse struct {
	Success   bool   `json:"success"`
	MessageID string `json:"message_id"`
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
