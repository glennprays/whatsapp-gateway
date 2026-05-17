package whatsapp

import (
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

const incomingBufferCapacity = 100

// IncomingMessage is an immutable record of a received WhatsApp message
// retained in the in-memory ring buffer. Field names match the existing
// webhook payload vocabulary so consumers see one consistent shape across
// webhook delivery and the polled /api/message/incoming endpoint.
//
// Once Pushed into a buffer, instances MUST be treated as read-only.
type IncomingMessage struct {
	MessageID string         `json:"message_id"`
	Chat      string         `json:"chat"`
	From      string         `json:"from"`
	IsGroup   bool           `json:"is_group"`
	PushName  string         `json:"push_name"`
	Timestamp int64          `json:"timestamp"`
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	Media     *IncomingMedia `json:"media,omitempty"`
}

// IncomingMedia is the metadata-only media descriptor stored in the buffer.
// The buffer never triggers media downloads on the capture goroutine, so URLs
// are intentionally not populated — use webhooks if you need fetchable links.
type IncomingMedia struct {
	Type     string `json:"type"`
	MimeType string `json:"mime_type,omitempty"`
	Size     uint64 `json:"size,omitempty"`
	FileName string `json:"file_name,omitempty"`
	Caption  string `json:"caption,omitempty"`
}

type incomingBuffer struct {
	mu   sync.RWMutex
	data []*IncomingMessage
	head int
	len  int
}

func newIncomingBuffer() *incomingBuffer {
	return &incomingBuffer{data: make([]*IncomingMessage, incomingBufferCapacity)}
}

func (b *incomingBuffer) Push(m *IncomingMessage) {
	if m == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data[b.head] = m
	b.head = (b.head + 1) % incomingBufferCapacity
	if b.len < incomingBufferCapacity {
		b.len++
	}
}

// Latest returns at most n most-recent messages, newest first. The returned
// slice is a fresh allocation; the pointed-to messages are shared and must be
// treated as read-only by callers.
func (b *incomingBuffer) Latest(n int) []*IncomingMessage {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if n > b.len {
		n = b.len
	}
	if n <= 0 {
		return []*IncomingMessage{}
	}
	out := make([]*IncomingMessage, n)
	idx := (b.head - 1 + incomingBufferCapacity) % incomingBufferCapacity
	for i := 0; i < n; i++ {
		out[i] = b.data[idx]
		idx = (idx - 1 + incomingBufferCapacity) % incomingBufferCapacity
	}
	return out
}

// toIncomingMessage converts a whatsmeow event into a buffered IncomingMessage.
// It mirrors the field shaping done by buildWebhookPayload but deliberately
// SKIPS any media download — only metadata directly available on the event
// proto is captured. Returns nil for events without basic info.
func toIncomingMessage(evt *events.Message, client *whatsmeow.Client) *IncomingMessage {
	if evt == nil {
		return nil
	}
	m := &IncomingMessage{
		MessageID: evt.Info.ID,
		Chat:      evt.Info.Chat.String(),
		From:      ConvertJIDToNonADLID(evt.Info.Sender, evt.Info.Chat, client),
		IsGroup:   evt.Info.IsGroup,
		PushName:  evt.Info.PushName,
		Timestamp: evt.Info.Timestamp.Unix(),
	}
	if evt.Message == nil {
		m.Type = "unknown"
		return m
	}
	switch {
	case evt.Message.Conversation != nil:
		m.Type = "text"
		m.Text = *evt.Message.Conversation
	case evt.Message.ExtendedTextMessage != nil:
		m.Type = "text"
		if t := evt.Message.ExtendedTextMessage.Text; t != nil {
			m.Text = *t
		}
	case evt.Message.ImageMessage != nil:
		m.Type = "image"
		m.Media = &IncomingMedia{
			Type:     "image",
			MimeType: evt.Message.ImageMessage.GetMimetype(),
			Size:     evt.Message.ImageMessage.GetFileLength(),
			Caption:  evt.Message.ImageMessage.GetCaption(),
		}
	case evt.Message.VideoMessage != nil:
		m.Type = "video"
		m.Media = &IncomingMedia{
			Type:     "video",
			MimeType: evt.Message.VideoMessage.GetMimetype(),
			Size:     evt.Message.VideoMessage.GetFileLength(),
			Caption:  evt.Message.VideoMessage.GetCaption(),
		}
	case evt.Message.AudioMessage != nil:
		m.Type = "audio"
		m.Media = &IncomingMedia{
			Type:     "audio",
			MimeType: evt.Message.AudioMessage.GetMimetype(),
			Size:     evt.Message.AudioMessage.GetFileLength(),
		}
	case evt.Message.DocumentMessage != nil:
		m.Type = "document"
		m.Media = &IncomingMedia{
			Type:     "document",
			MimeType: evt.Message.DocumentMessage.GetMimetype(),
			Size:     evt.Message.DocumentMessage.GetFileLength(),
			FileName: evt.Message.DocumentMessage.GetFileName(),
		}
	case evt.Message.StickerMessage != nil:
		m.Type = "sticker"
		m.Media = &IncomingMedia{
			Type:     "sticker",
			MimeType: evt.Message.StickerMessage.GetMimetype(),
		}
	case evt.Message.ContactMessage != nil:
		m.Type = "contact"
	case evt.Message.LocationMessage != nil:
		m.Type = "location"
	default:
		m.Type = "unknown"
	}
	return m
}
