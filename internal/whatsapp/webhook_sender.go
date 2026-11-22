package whatsapp

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/metrics"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	"github.com/google/uuid"
)

// maxWebhookRetries caps configured retries so a misconfigured budget can't push
// the detached-context timeout (and backoff) to absurd values.
const maxWebhookRetries = 10

var errWebhookDropped = errors.New("webhook dropped: delivery semaphore full")

type WebhookSender struct {
	cipher     *cipherx.Cipher
	logger     *customLog.Logger
	httpClient *http.Client
	maxRetries int
	backoff    time.Duration
	sem        chan struct{}
	dlq        *deadLetterStore
}

func NewWebhookSender(cipher *cipherx.Cipher, logger *customLog.Logger, cfg *config.Config, db *sql.DB) *WebhookSender {
	maxRetries := cfg.WebhookMaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > maxWebhookRetries {
		maxRetries = maxWebhookRetries
	}
	backoff := time.Duration(cfg.WebhookRetryBackoffSeconds) * time.Second
	if backoff <= 0 {
		backoff = 2 * time.Second
	}
	sender := &WebhookSender{
		cipher:     cipher,
		logger:     logger,
		maxRetries: maxRetries,
		backoff:    backoff,
		sem:        make(chan struct{}, maxConcurrentWebhooks),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
			Transport: &http.Transport{
				DialContext: ssrfSafeDialContext,
			},
		},
	}
	if db != nil {
		sender.dlq = newDeadLetterStore(db, cipher)
	}
	return sender
}

func ssrfSafeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}

	for _, ip := range ips {
		if utils.IsPrivateIP(ip) {
			return nil, fmt.Errorf("blocked SSRF: resolved to private IP %s", ip)
		}
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

func (ws *WebhookSender) Send(ctx context.Context, url string, encryptedHmacSecret string, payload interface{}) error {
	// Marshal payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Decrypt HMAC secret. An empty stored secret means the subscription is
	// unsigned: sign with an empty key (identical to decrypting the legacy
	// ciphertext-of-"" that pre-multi-URL webhooks stored).
	hmacSecret := ""
	if encryptedHmacSecret != "" {
		hmacSecret, err = ws.cipher.Decrypt(encryptedHmacSecret)
		if err != nil {
			ws.logger.Error("", "Failed to decrypt HMAC secret", nil, customLog.Error(err))
			return fmt.Errorf("failed to decrypt HMAC secret: %w", err)
		}
	}

	// Generate HMAC-SHA256 signature
	h := hmac.New(sha256.New, []byte(hmacSecret))
	h.Write(jsonPayload)
	signature := hex.EncodeToString(h.Sum(nil))

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", fmt.Sprintf("sha256=%s", signature))
	req.Header.Set("User-Agent", "WhatsApp-Gateway-Webhook/1.0")

	// Send request
	resp, err := ws.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}

// SendAsync delivers a webhook off the request path with bounded retry. It is
// fire-and-forget and semaphore-bounded. On saturation the delivery is NOT
// dropped: it is persisted synchronously as an encrypted dead letter so the
// operator can replay it. After exhausted retries the failed delivery is also
// persisted (encrypted) for seven days.
//
// This is a dead-letter store, not an exactly-once durable outbox: a crash
// before the failed delivery is written, or database unavailability, can still
// lose it. Persistence failures are logged (non-sensitive classification only)
// and counted by the direct failure metric; plaintext is never stored.
//
// Critical: the goroutine builds its OWN context from context.Background(), NOT
// the request ctx. The request ctx is cancelled the instant the HTTP response is
// written, so a naive `go ws.Send(reqCtx, ...)` would fail every delivery.
func (ws *WebhookSender) SendAsync(url, encryptedHmacSecret, event string, payload interface{}) {
	traceID := uuid.NewString() // detached delivery has no request trace
	select {
	case ws.sem <- struct{}{}:
		go func() {
			defer func() { <-ws.sem }()
			ctx, cancel := context.WithTimeout(context.Background(), ws.retryBudget())
			defer cancel()
			err := ws.sendWithRetry(ctx, url, encryptedHmacSecret, payload)
			metrics.RecordWebhook("direct", event, err)
			if err != nil {
				ws.logger.Error(traceID, "Webhook delivery failed after retries", nil,
					customLog.String("event", event), customLog.Error(err))
				ws.persistDeadLetter(traceID, deadLetterDeliveryFailed, url, encryptedHmacSecret, event, payload)
			}
		}()
	default:
		metrics.RecordWebhook("direct", event, errWebhookDropped)
		ws.logger.Warn(traceID, "Webhook delivery semaphore full, parking "+event+" as dead letter", nil)
		// Synchronous persist so a saturated sender loses nothing; the
		// caller may block briefly on the DB write.
		ws.persistDeadLetter(traceID, deadLetterSemaphoreFull, url, encryptedHmacSecret, event, payload)
	}
}

// persistDeadLetter serializes, encrypts and inserts a dead letter. The
// payload map must be marshallable; failures are classified, logged without
// sensitive content, and counted — never exposed, never stored plaintext.
func (ws *WebhookSender) persistDeadLetter(traceID, classification, url, encryptedHmacSecret, event string, payload interface{}) {
	if ws.dlq == nil {
		metrics.RecordWebhookDeadLetterMiss()
		return
	}
	payloadMap, ok := payload.(map[string]interface{})
	if !ok {
		classification = deadLetterInvalidPayload
		payloadMap = map[string]interface{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now()
	err := ws.dlq.Insert(ctx, uuid.NewString(), event, classification, DeadLetterDelivery{
		URL:        url,
		HmacSecret: encryptedHmacSecret,
		Payload:    payloadMap,
	}, now)
	if err != nil {
		ws.logger.Error(traceID, "Failed to persist webhook dead letter", nil,
			customLog.String("event", event), customLog.String("classification", classification))
		metrics.RecordWebhookDeadLetterMiss()
	}
}

// ErrDeadLetterStoreDisabled reports no DB-backed store on this sender.
var ErrDeadLetterStoreDisabled = errors.New("dead letter store not configured")

// ListDeadLetters returns unexpired dead letters (non-sensitive projection),
// newest first, after purging expired rows.
func (ws *WebhookSender) ListDeadLetters(ctx context.Context, limit, offset int) ([]DeadLetterItem, error) {
	if ws.dlq == nil {
		return nil, ErrDeadLetterStoreDisabled
	}
	now := time.Now()
	if err := ws.dlq.DeleteExpired(ctx, now); err != nil {
		return nil, err
	}
	return ws.dlq.List(ctx, limit, offset, now)
}

// ReplayDeadLetter atomically claims an unexpired dead letter, decrypts it,
// and sends exactly one HTTP attempt through the SSRF-safe Send (no redirect
// following, original HMAC signing). On 2xx the row is deleted (only when our
// lease still matches) and the caller reports delivered; on any failure the
// lease is cleared, the row retained, and the caller reports failed.
//
// A crashed replay becomes claimable after DeadLetterLeaseSeconds; a crash
// after the remote 2xx but before delete can duplicate the delivery, so replay
// is at-least-once. Never surfaces envelope contents in returned errors.
func (ws *WebhookSender) ReplayDeadLetter(ctx context.Context, id string) error {
	if ws.dlq == nil {
		return ErrDeadLetterStoreDisabled
	}
	now := time.Now()
	if err := ws.dlq.DeleteExpired(ctx, now); err != nil {
		return err
	}
	token := uuid.NewString()
	if err := ws.dlq.ClaimLease(ctx, id, token, now); err != nil {
		return err
	}
	delivery, _, err := ws.dlq.LoadForReplay(ctx, id)
	if err != nil {
		_ = ws.dlq.ReleaseLease(ctx, id, token, time.Now())
		return err
	}
	if err := ws.Send(ctx, delivery.URL, delivery.HmacSecret, delivery.Payload); err != nil {
		_ = ws.dlq.ReleaseLease(ctx, id, token, time.Now())
		return err
	}
	return ws.dlq.DeleteLeased(ctx, id, token)
}

// sendWithRetry attempts delivery up to maxRetries+1 times with bounded
// exponential backoff, aborting early if the context is cancelled.
func (ws *WebhookSender) sendWithRetry(ctx context.Context, url, encryptedHmacSecret string, payload interface{}) error {
	var err error
	for attempt := 0; attempt <= ws.maxRetries; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(ws.backoff << (attempt - 1))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		if err = ws.Send(ctx, url, encryptedHmacSecret, payload); err == nil {
			return nil
		}
	}
	return err
}

// retryBudget bounds the detached context so it covers every attempt (each with
// its own ~5s http timeout) plus the exponential backoff between them.
func (ws *WebhookSender) retryBudget() time.Duration {
	budget := time.Duration(0)
	for attempt := 0; attempt <= ws.maxRetries; attempt++ {
		budget += 6 * time.Second
		if attempt > 0 {
			budget += ws.backoff << (attempt - 1)
		}
	}
	return budget
}
