package whatsapp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// blockingDownloader blocks until its context is cancelled, proving the
// download deadline comes from the injected mediaTimeout.
type blockingDownloader struct {
	calls    int
	mu       sync.Mutex
	gotStart chan struct{}
}

func (b *blockingDownloader) DownloadAndStoreMedia(ctx context.Context, traceID, phoneNumber string, media DownloadableMessage, mediaType string) (string, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	if b.gotStart != nil {
		b.gotStart <- struct{}{}
	}
	<-ctx.Done()
	return "", ctx.Err()
}
func contactCardEvent() *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("6281", types.DefaultUserServer),
				Chat:   types.NewJID("6282", types.DefaultUserServer),
			},
			ID:        "msgid-1",
			Timestamp: time.Unix(1730000000, 0),
			PushName:  "Bob",
		},
		Message: &proto.Message{
			ContactMessage: &proto.ContactMessage{
				DisplayName: new("Alice"),
				Vcard:       new("BEGIN:VCARD\nEND:VCARD"),
			},
		},
	}
}

// TestBuildWebhookPayloadContactCard locks the direct-mode payload shape for a
// contact card — the case that previously diverged between direct and queue
// modes (queue mode had no ContactMessage branch at all).
func TestBuildWebhookPayloadContactCard(t *testing.T) {
	msg := contactCardEvent()
	payload := BuildWebhookPayload(msg, nil, "trace-1", "6282", nil, 5*time.Second)

	if payload["event"] != "message.incoming" {
		t.Fatalf("event = %v", payload["event"])
	}
	if payload["type"] != "contact" {
		t.Fatalf("type = %v, want contact", payload["type"])
	}
	if payload["display_name"] != "Alice" {
		t.Fatalf("display_name = %v, want Alice", payload["display_name"])
	}
	if payload["message_id"] != "msgid-1" {
		t.Fatalf("message_id = %v", payload["message_id"])
	}
	if _, hasMedia := payload["media"]; hasMedia {
		t.Fatal("contact card must not carry a media block")
	}

	// The payload must marshal cleanly for webhook delivery.
	if _, err := json.Marshal(payload); err != nil {
		t.Fatalf("payload not marshallable: %v", err)
	}
}

// TestBuildWebhookPayloadTextAndUnknown covers the text branch and the
// nil-Message early return (nil-message handling).
func TestBuildWebhookPayloadTextAndUnknown(t *testing.T) {
	msg := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("6281", types.DefaultUserServer),
				Chat:   types.NewJID("6282", types.DefaultUserServer),
			},
			ID:        "id-2",
			Timestamp: time.Unix(1730000001, 0),
		},
		Message: &proto.Message{Conversation: new("hello")},
	}
	p := BuildWebhookPayload(msg, nil, "t", "6282", nil, 5*time.Second)
	if p["type"] != "text" || p["text"] != "hello" {
		t.Fatalf("text payload = %v/%v", p["type"], p["text"])
	}

	nilMsg := &events.Message{Info: msg.Info}
	np := BuildWebhookPayload(nilMsg, nil, "t", "6282", nil, 5*time.Second)
	if _, has := np["type"]; has {
		t.Fatal("nil Message must produce no type field")
	}
	if np["message_id"] != "id-2" {
		t.Fatalf("message_id = %v", np["message_id"])
	}
}

// TestBuildWebhookPayloadMediaFailureFields covers the media-error path: when
// the download fails, the media block still exists with the WhatsApp URL.
func TestBuildWebhookPayloadMediaFailureFields(t *testing.T) {
	msg := &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("6281", types.DefaultUserServer),
				Chat:   types.NewJID("6282", types.DefaultUserServer),
			},
			ID:        "id-3",
			Timestamp: time.Unix(1730000002, 0),
		},
		Message: &proto.Message{
			ImageMessage: &proto.ImageMessage{
				URL:      new("https://mmg.example/img.jpeg"),
				Mimetype: new("image/jpeg"),
			},
		},
	}
	// Blocking downloader + short timeout => download fails via ctx deadline.
	bd := &blockingDownloader{}
	start := time.Now()
	p := BuildWebhookPayload(msg, bd, "t", "6282", nil, 50*time.Millisecond)
	if time.Since(start) < 40*time.Millisecond {
		t.Fatal("media download returned before the injected timeout")
	}
	if p["type"] != "image" {
		t.Fatalf("type = %v", p["type"])
	}
	media, ok := p["media"].(map[string]interface{})
	if !ok {
		t.Fatalf("media block missing: %#v", p["media"])
	}
	if media["url"] != "https://mmg.example/img.jpeg" {
		t.Fatalf("fallback url = %v", media["url"])
	}
}

// TestDownloadMediaTimeoutBoundsCancellation proves the injected timeout is
// the download deadline: a blocking downloader is released at ~that deadline.
func TestDownloadMediaTimeoutBoundsCancellation(t *testing.T) {
	bd := &blockingDownloader{gotStart: make(chan struct{}, 1)}
	start := time.Now()
	_, err := downloadMedia(bd, "t", "6282", &proto.ImageMessage{}, "image", 80*time.Millisecond)
	if err == nil {
		t.Fatal("expected ctx deadline error")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	elapsed := time.Since(start)
	if elapsed < 60*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("download cancelled after %v, want ~80ms", elapsed)
	}
}

// TestDownloadMediaZeroTimeoutFallsBackTo5s: <= 0 keeps the historical default.
func TestDownloadMediaZeroTimeoutFallsBackTo5s(t *testing.T) {
	bd := &blockingDownloader{gotStart: make(chan struct{}, 1)}
	start := time.Now()
	// 0 must normalize to 5s; assert we are NOT cancelled early. The point is
	// that 0 did not mean "immediate".
	done := make(chan error, 1)
	go func() {
		_, err := downloadMedia(bd, "t", "6282", &proto.ImageMessage{}, "image", 0)
		done <- err
	}()
	<-bd.gotStart
	// Give the download 300ms; a zero-timeout bug would cancel immediately.
	select {
	case <-done:
		t.Fatal("download with 0 timeout finished early; 0 must fall back to 5s")
	case <-time.After(300 * time.Millisecond):
	}
	// Now wait for the real 5s fallback to fire (bounded by test timeout).
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("err = %v, want deadline exceeded after 5s fallback", err)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("5s fallback deadline never fired")
	}
	if time.Since(start) < 4*time.Second {
		t.Fatalf("0 timeout cancelled at %v; want ~5s fallback", time.Since(start))
	}
}
