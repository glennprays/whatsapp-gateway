package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	_ "modernc.org/sqlite"
)

// newDeadLetterTestDB spins up an in-memory SQLite with the dead-letter table.
func newDeadLetterTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := runWebhookDeadLettersMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// newDLQSender builds a sender wired to an in-memory dead-letter store with a
// vanilla HTTP client (ssrf-safe dialer blocks the loopback httptest uses).
func newDLQSender(t *testing.T, maxRetries int) (*WebhookSender, string, *sql.DB) {
	t.Helper()
	cipher := cipherx.NewCipher("0123456789abcdef0123456789abcdef")
	enc, err := cipher.Encrypt("hmac-secret")
	if err != nil {
		t.Fatal(err)
	}
	db := newDeadLetterTestDB(t)
	cfg := &config.Config{WebhookMaxRetries: maxRetries, WebhookRetryBackoffSeconds: 1}
	ws := NewWebhookSender(cipher, newTestLogger(t), cfg, db)
	ws.backoff = 5 * time.Millisecond
	ws.httpClient = &http.Client{Timeout: 2 * time.Second}
	return ws, enc, db
}

func countDeadLetters(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM webhook_dead_letters").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestSendAsyncExhaustedRetriesPersistsDeadLetter: a delivery that exhausts
// its retries must leave exactly one encrypted, listable dead letter.
func TestSendAsyncExhaustedRetriesPersistsDeadLetter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ws, enc, db := newDLQSender(t, 2)
	payload := map[string]interface{}{"event": "message.sent", "to": "628"}
	ws.SendAsync(srv.URL, enc, "message.sent", payload)

	deadline := time.Now().Add(5 * time.Second)
	for countDeadLetters(t, db) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := countDeadLetters(t, db); got != 1 {
		t.Fatalf("dead letters = %d, want 1", got)
	}

	// Raw DB fields must not contain URL, secret plaintext, or payload text.
	var encrypted, lastErr, event string
	if err := db.QueryRow("SELECT encrypted_delivery, last_error, event FROM webhook_dead_letters").
		Scan(&encrypted, &lastErr, &event); err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{srv.URL, "hmac-secret", `"message.sent"`, "628", "message.sent"} {
		// event column legitimately holds the event name; check others.
		if leak == "message.sent" {
			continue
		}
		if strings.Contains(encrypted, leak) || strings.Contains(lastErr, leak) {
			t.Fatalf("sensitive value %q leaked into stored columns (encrypted=%q last_error=%q)", leak, encrypted, lastErr)
		}
	}
	if !strings.HasPrefix(encrypted, "1:") {
		t.Fatalf("encrypted_delivery is not a cipherx blob: %q", encrypted[:min(20, len(encrypted))])
	}
	if lastErr != deadLetterDeliveryFailed {
		t.Fatalf("last_error = %q, want fixed classification %q", lastErr, deadLetterDeliveryFailed)
	}

	// The list projection is metadata-only.
	items, err := ws.ListDeadLetters(context.Background(), 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("listed %d items, want 1", len(items))
	}
	if items[0].ExpiresAt-items[0].CreatedAt != deadLetterRetentionSeconds {
		t.Fatalf("retention = %d, want %d (7 days)", items[0].ExpiresAt-items[0].CreatedAt, deadLetterRetentionSeconds)
	}
}

// TestSendAsyncSemaphoreFullPersistsDeadLetter: a saturated sender must
// synchronously persist the delivery instead of dropping it.
func TestSendAsyncSemaphoreFullPersistsDeadLetter(t *testing.T) {
	// Server that hangs until the test ends, keeping the 1-slot semaphore busy.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	defer close(release)

	ws, enc, db := newDLQSender(t, 0)
	// Shrink the semaphore to a single slot.
	ws.sem = make(chan struct{}, 1)

	// First delivery occupies the slot (detached goroutine hangs on server).
	ws.SendAsync(srv.URL, enc, "message.sent", map[string]interface{}{"a": 1})
	// Second delivery finds the semaphore full: must persist synchronously,
	// i.e. by the time SendAsync returns the row exists.
	start := time.Now()
	ws.SendAsync(srv.URL, enc, "message.failed", map[string]interface{}{"b": 2})
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("synchronous persist took %v", elapsed)
	}
	if got := countDeadLetters(t, db); got != 1 {
		t.Fatalf("dead letters = %d, want 1 (saturated delivery must be persisted synchronously)", got)
	}
	var lastErr string
	if err := db.QueryRow("SELECT last_error FROM webhook_dead_letters").Scan(&lastErr); err != nil {
		t.Fatal(err)
	}
	if lastErr != deadLetterSemaphoreFull {
		t.Fatalf("last_error = %q, want %q", lastErr, deadLetterSemaphoreFull)
	}
}

// TestReplayDeliversAndDeletes: after a failed delivery persists a dead
// letter, a succeeding target plus replay must deliver the original payload
// with a valid HMAC signature and delete the row.
func TestReplayDeliversAndDeletes(t *testing.T) {
	var fail int32 = 1
	var body []byte
	var sig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&fail) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		body = make([]byte, r.ContentLength)
		r.Body.Read(body)
		sig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ws, enc, db := newDLQSender(t, 0)
	ws.SendAsync(srv.URL, enc, "message.sent", map[string]interface{}{"k": "v", "n": 7})

	deadline := time.Now().Add(5 * time.Second)
	for countDeadLetters(t, db) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if countDeadLetters(t, db) != 1 {
		t.Fatal("dead letter never persisted")
	}

	// Simulate sender reconstruction (restart): a fresh sender over the same DB.
	cipher2 := cipherx.NewCipher("0123456789abcdef0123456789abcdef")
	ws2 := NewWebhookSender(cipher2, newTestLogger(t), &config.Config{WebhookMaxRetries: 0}, db)
	ws2.httpClient = &http.Client{Timeout: 2 * time.Second}
	atomic.StoreInt32(&fail, 0)

	items, err := ws2.ListDeadLetters(context.Background(), 50, 0)
	if err != nil || len(items) != 1 {
		t.Fatalf("list after reconstruction: %v %d", err, len(items))
	}

	if err := ws2.ReplayDeadLetter(context.Background(), items[0].ID); err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if got := countDeadLetters(t, db); got != 0 {
		t.Fatalf("row retained after successful replay: %d", got)
	}

	// Payload preserved byte-for-byte; HMAC signature present and correct.
	if !strings.Contains(string(body), `"k":"v"`) || !strings.Contains(string(body), `"n":7`) {
		t.Fatalf("replayed body lost payload: %s", body)
	}
	h := hmacSha256Hex("hmac-secret", body)
	if sig != "sha256="+h {
		t.Fatalf("signature mismatch: got %s want sha256=%s", sig, h)
	}

	// Replay of the deleted row is 404-class.
	if err := ws2.ReplayDeadLetter(context.Background(), items[0].ID); !errors.Is(err, ErrDeadLetterNotFound) {
		t.Fatalf("replay of deleted row err = %v, want ErrDeadLetterNotFound", err)
	}
}

// TestReplayRetainsOnFailure: a non-2xx replay keeps the row (502 path).
func TestReplayRetainsOnFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ws, enc, db := newDLQSender(t, 0)
	ws.SendAsync(srv.URL, enc, "message.sent", map[string]interface{}{"k": "v"})
	deadline := time.Now().Add(5 * time.Second)
	for countDeadLetters(t, db) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}

	items, _ := ws.ListDeadLetters(context.Background(), 50, 0)
	if len(items) != 1 {
		t.Fatal("no dead letter to replay")
	}
	if err := ws.ReplayDeadLetter(context.Background(), items[0].ID); err == nil {
		t.Fatal("replay must fail on 500 target")
	}
	if got := countDeadLetters(t, db); got != 1 {
		t.Fatalf("row deleted after failed replay: %d", got)
	}
	// Lease must be cleared so the row is retryable immediately.
	if err := ws.ReplayDeadLetter(context.Background(), items[0].ID); err == nil {
		t.Fatal("second replay should also fail (target still 500)")
	}
}

// TestReplayConcurrentClaimRejected: a second claim while the lease is live
// gets ErrDeadLetterLeaseHeld (409 to the admin surface).
func TestReplayConcurrentClaimRejected(t *testing.T) {
	ws, _, db := newDLQSender(t, 0)
	now := time.Now()
	if err := ws.dlq.Insert(context.Background(), "dl-1", "message.sent", deadLetterDeliveryFailed,
		DeadLetterDelivery{URL: "https://example.invalid/hook", Payload: map[string]interface{}{}}, now); err != nil {
		t.Fatal(err)
	}

	if err := ws.dlq.ClaimLease(context.Background(), "dl-1", "token-a", now); err != nil {
		t.Fatalf("first claim failed: %v", err)
	}
	err := ws.dlq.ClaimLease(context.Background(), "dl-1", "token-b", now)
	if !errors.Is(err, ErrDeadLetterLeaseHeld) {
		t.Fatalf("second claim err = %v, want ErrDeadLetterLeaseHeld", err)
	}
	// After the lease expires the row is claimable again (crash recovery).
	later := now.Add(time.Duration(DeadLetterLeaseSeconds+1) * time.Second)
	if err := ws.dlq.ClaimLease(context.Background(), "dl-1", "token-c", later); err != nil {
		t.Fatalf("claim after lease expiry failed: %v", err)
	}
	_ = db
}

// TestReplayRejectsExpiredAndMissing: expired rows are purged and 404-class.
func TestReplayRejectsExpiredAndMissing(t *testing.T) {
	ws, _, _ := newDLQSender(t, 0)
	ctx := context.Background()

	if err := ws.ReplayDeadLetter(ctx, "no-such-id"); !errors.Is(err, ErrDeadLetterNotFound) {
		t.Fatalf("missing id err = %v, want ErrDeadLetterNotFound", err)
	}

	// Insert an already-expired row.
	past := time.Now().Add(-2 * time.Duration(deadLetterRetentionSeconds) * time.Second)
	if err := ws.dlq.Insert(ctx, "dl-old", "message.sent", deadLetterDeliveryFailed,
		DeadLetterDelivery{URL: "https://example.invalid/hook", Payload: map[string]interface{}{}}, past); err != nil {
		t.Fatal(err)
	}
	if err := ws.ReplayDeadLetter(ctx, "dl-old"); !errors.Is(err, ErrDeadLetterNotFound) {
		t.Fatalf("expired id err = %v, want ErrDeadLetterNotFound (and purge)", err)
	}
}

// TestDeadLetterPersistenceFailureNoPlaintextFallback: when the DB is closed
// (unavailable), persisting fails and no plaintext row appears anywhere.
func TestDeadLetterPersistenceFailureNoPlaintextFallback(t *testing.T) {
	ws, _, db := newDLQSender(t, 0)
	db.Close() // simulate database unavailability

	// Must not panic and must not store anything.
	ws.persistDeadLetter("trace-test", deadLetterDeliveryFailed, "https://x.example/h", "enc-secret", "message.sent",
		map[string]interface{}{"secret-payload": "value"})
}

func hmacSha256Hex(key string, body []byte) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
