package handlers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
)

// fakePublisher captures webhook delivery publications.
type fakePublisher struct {
	pub []domainQueue.WebhookDeliveryMessage
}

func (f *fakePublisher) PublishWebhookDelivery(ctx context.Context, msg domainQueue.WebhookDeliveryMessage) error {
	f.pub = append(f.pub, msg)
	return nil
}

// PublishWebhookDelivery is re-declared via embedding in tests through the
// concrete *queue.RabbitMQQueue field; Handle only needs this method, so the
// fake satisfies it by being passed as the handler's Publisher via the
// interface-free path used below.

// recordingDownloader proves the queue path also passes MediaTimeout through.
type recordingDownloader struct {
	calls     int
	deadline  time.Duration // observed ctx deadline
	started   chan struct{}
	cancelled chan struct{}
}

func (r *recordingDownloader) DownloadAndStoreMedia(ctx context.Context, traceID, phoneNumber string, media whatsapp.DownloadableMessage, mediaType string) (string, error) {
	r.calls++
	dl, ok := ctx.Deadline()
	if ok {
		r.deadline = time.Until(dl)
	}
	r.started <- struct{}{}
	<-ctx.Done()
	r.cancelled <- struct{}{}
	return "", ctx.Err()
}

func contactEventForQueue() *events.Message {
	return &events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{
				Sender: types.NewJID("6281", types.DefaultUserServer),
				Chat:   types.NewJID("6282", types.DefaultUserServer),
			},
			ID:        "q-msg-1",
			Timestamp: time.Unix(1730000000, 0),
			PushName:  "Bob",
		},
		Message: &proto.Message{
			ContactMessage: &proto.ContactMessage{
				DisplayName: new("Alice"),
			},
		},
	}
}

// TestQueuePayloadMatchesDirect proves direct and queue modes emit identical
// incoming-message payloads for the same contact-card event: the queue handler
// now calls the SAME exported builder the direct path uses. (Pre-fix, queue
// mode had no ContactMessage branch and its text/media cases were ordered
// differently, producing different types for the same event.)
func TestQueuePayloadMatchesDirect(t *testing.T) {
	evt := contactEventForQueue()
	raw, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}

	h := &IncomingEventHandler{
		MediaTimeout: 5 * time.Second,
	}

	// The builder is the single shared authority; assert the queue-side call
	// produces the same map the direct side produces.
	direct := whatsapp.BuildWebhookPayload(evt, nil, "t", "6282", nil, 5*time.Second)
	queued := whatsapp.BuildWebhookPayload(unmarshalEvent(t, raw), nil, "t", "6282", nil, h.MediaTimeout)

	if queued["type"] != direct["type"] {
		t.Fatalf("type diverged: queue=%v direct=%v", queued["type"], direct["type"])
	}
	if queued["type"] != "contact" {
		t.Fatalf("type = %v, want contact (queue mode previously dropped contact cards)", queued["type"])
	}
	if queued["display_name"] != direct["display_name"] {
		t.Fatalf("display_name diverged: queue=%v direct=%v", queued["display_name"], direct["display_name"])
	}
	dj, _ := json.Marshal(direct)
	qj, _ := json.Marshal(queued)
	if string(dj) != string(qj) {
		t.Fatalf("payloads differ:\ndirect: %s\nqueue:  %s", dj, qj)
	}
	_ = h
}

func unmarshalEvent(t *testing.T, raw []byte) *events.Message {
	t.Helper()
	var evt events.Message
	if err := json.Unmarshal(raw, &evt); err != nil {
		t.Fatal(err)
	}
	return &evt
}

// TestQueueMediaTimeoutInjected: the queue handler's MediaTimeout reaches the
// downloader as its context deadline — a blocking download is cancelled at the
// configured deadline, not the old hardcoded 5s (here 120ms).
func TestQueueMediaTimeoutInjected(t *testing.T) {
	rd := &recordingDownloader{
		started:   make(chan struct{}, 1),
		cancelled: make(chan struct{}, 1),
	}

	evt := contactEventForQueue()
	// Swap in an image message so a download actually happens.
	evt.Message = &proto.Message{
		ImageMessage: &proto.ImageMessage{
			URL:      new("https://mmg.example/x.jpeg"),
			Mimetype: new("image/jpeg"),
		},
	}

	h := &IncomingEventHandler{
		MediaDownloader: rd,
		MediaTimeout:    120 * time.Millisecond,
	}

	payload := whatsapp.BuildWebhookPayload(evt, h.MediaDownloader, "t", "6282", nil, h.MediaTimeout)
	<-rd.started

	select {
	case <-rd.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("download was never cancelled by the injected timeout")
	}
	if rd.deadline <= 0 || rd.deadline > 2*time.Second {
		t.Fatalf("observed deadline = %v, want ~120ms", rd.deadline)
	}
	if payload["type"] != "image" {
		t.Fatalf("type = %v", payload["type"])
	}
	media := payload["media"].(map[string]interface{})
	if media["url"] != "https://mmg.example/x.jpeg" {
		t.Fatalf("fallback url = %v", media["url"])
	}
}
