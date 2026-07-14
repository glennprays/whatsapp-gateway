package whatsapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
)

// newTestSender builds a sender with a vanilla http client (the production
// ssrfSafeDialContext blocks loopback, which httptest listens on) and a tiny
// backoff so retry tests stay fast.
func newTestSender(t *testing.T, maxRetries int) (*WebhookSender, string) {
	t.Helper()
	cipher := cipherx.NewCipher("0123456789abcdef0123456789abcdef")
	enc, err := cipher.Encrypt("hmac-secret")
	if err != nil {
		t.Fatal(err)
	}
	ws := &WebhookSender{
		cipher:     cipher,
		logger:     newTestLogger(t),
		httpClient: &http.Client{Timeout: 2 * time.Second},
		maxRetries: maxRetries,
		backoff:    5 * time.Millisecond,
		sem:        make(chan struct{}, maxConcurrentWebhooks),
	}
	return ws, enc
}

func TestSendWithRetrySucceedsAfterFailures(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ws, enc := newTestSender(t, 3)
	if err := ws.sendWithRetry(context.Background(), srv.URL, enc, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestSendWithRetryGivesUp(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ws, enc := newTestSender(t, 2)
	if err := ws.sendWithRetry(context.Background(), srv.URL, enc, map[string]string{"k": "v"}); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&attempts); got != 3 { // maxRetries(2) + 1
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestSendAsyncNonBlockingAndDetached(t *testing.T) {
	done := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(80 * time.Millisecond) // slow handler
		w.WriteHeader(http.StatusOK)
		select {
		case done <- struct{}{}:
		default:
		}
	}))
	defer srv.Close()

	ws, enc := newTestSender(t, 0)

	start := time.Now()
	ws.SendAsync(srv.URL, enc, "message.sent", map[string]string{"k": "v"})
	if elapsed := time.Since(start); elapsed > 40*time.Millisecond {
		t.Fatalf("SendAsync blocked for %v, want immediate return", elapsed)
	}

	// The detached (context.Background-derived) ctx must let delivery complete
	// even though no request context is keeping it alive.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("webhook was never delivered by the detached goroutine")
	}
}
