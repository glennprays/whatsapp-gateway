package whatsapp

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	"net/http"
	"time"
)

// Dead-letter classifications. last_error stores ONLY these fixed strings so a
// leaked log or admin response can never carry transport error text.
const (
	deadLetterDeliveryFailed = "delivery_failed"
	deadLetterSemaphoreFull  = "semaphore_full"
	deadLetterInvalidPayload = "invalid_payload"
)

// deadLetterRetentionSeconds is exactly 7 days; expiry is created_at + this
// constant and is never extended by replay.
const deadLetterRetentionSeconds int64 = 604800

// DeadLetterLeaseSeconds bounds how long a replay attempt may hold a row.
const DeadLetterLeaseSeconds int64 = 300

// DeadLetterDelivery is the JSON envelope encrypted into
// webhook_dead_letters.encrypted_delivery. HmacSecret stays the existing
// encrypted subscription secret (it is only decrypted inside Send at delivery
// time), so the envelope never contains plaintext credentials.
type DeadLetterDelivery struct {
	URL        string                 `json:"url"`
	HmacSecret string                 `json:"hmac_secret"`
	Payload    map[string]interface{} `json:"payload"`
}

// DeadLetterItem is the non-sensitive projection returned by List. It never
// includes the encrypted delivery, URL, secret, payload, or error text.
type DeadLetterItem struct {
	ID        string `json:"id"`
	Event     string `json:"event"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// deadLetterStore persists failed direct-mode webhook deliveries. It is a
// dead-letter store, NOT an exactly-once durable outbox: a crash before the
// failed delivery is written, or database unavailability, can still lose it.
type deadLetterStore struct {
	db     *sql.DB
	cipher *cipherx.Cipher
}

func newDeadLetterStore(db *sql.DB, cipher *cipherx.Cipher) *deadLetterStore {
	return &deadLetterStore{db: db, cipher: cipher}
}

// DeleteExpired removes rows past expires_at. Called at startup and before
// every list/replay so expiry is enforced without a background timer.
func (s *deadLetterStore) DeleteExpired(ctx context.Context, now time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM webhook_dead_letters WHERE expires_at <= $1`,
		now.Unix(),
	)
	return err
}

// Insert serializes + encrypts the delivery and stores it. On any failure it
// returns the error; the caller logs (non-sensitive) and counts the miss.
// Plaintext is never stored as a fallback.
func (s *deadLetterStore) Insert(ctx context.Context, id, event, classification string, delivery DeadLetterDelivery, now time.Time) error {
	plain, err := json.Marshal(delivery)
	if err != nil {
		return fmt.Errorf("serialize dead letter: %w", err)
	}
	encrypted, err := s.cipher.Encrypt(string(plain))
	if err != nil {
		return fmt.Errorf("encrypt dead letter: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
        INSERT INTO webhook_dead_letters
            (id, created_at, expires_at, event, encrypted_delivery, last_error, lease_token, lease_until)
        VALUES ($1, $2, $3, $4, $5, $6, '', 0)
        ON CONFLICT (id) DO NOTHING`,
		id, now.Unix(), now.Unix()+deadLetterRetentionSeconds, event, encrypted, classification,
	)
	return err
}

// List returns unexpired rows ordered by (created_at, id) DESC, newest first.
func (s *deadLetterStore) List(ctx context.Context, limit, offset int, now time.Time) ([]DeadLetterItem, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, event, created_at, expires_at
        FROM webhook_dead_letters
        WHERE expires_at > $1
        ORDER BY created_at DESC, id DESC
        LIMIT $2 OFFSET $3`,
		now.Unix(), limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]DeadLetterItem, 0, limit)
	for rows.Next() {
		var it DeadLetterItem
		if err := rows.Scan(&it.ID, &it.Event, &it.CreatedAt, &it.ExpiresAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ErrDeadLetterLeaseHeld reports an unexpired replay lease on the row.
var ErrDeadLetterLeaseHeld = errors.New("dead letter lease already held")

// ErrDeadLetterNotFound covers missing AND expired IDs (both 404 to callers;
// expiry is indistinguishable from absence by design).
var ErrDeadLetterNotFound = errors.New("dead letter not found or expired")

// ClaimLease atomically takes the replay lease on an unexpired row: only when
// no unexpired lease exists, it sets lease_token/lease_until. A crashed replay
// becomes claimable again after DeadLetterLeaseSeconds.
func (s *deadLetterStore) ClaimLease(ctx context.Context, id string, token string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `
        UPDATE webhook_dead_letters
        SET lease_token = $1, lease_until = $2
        WHERE id = $3
          AND expires_at > $4
          AND (lease_until <= $4 OR lease_token = '')`,
		token, now.Unix()+DeadLetterLeaseSeconds, id, now.Unix(),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// Missing, expired, or leased: distinguish leased for the 409.
		var leaseUntil int64
		err := s.db.QueryRowContext(ctx,
			`SELECT lease_until FROM webhook_dead_letters WHERE id = $1 AND expires_at > $2`,
			id, now.Unix(),
		).Scan(&leaseUntil)
		if err != nil {
			return ErrDeadLetterNotFound
		}
		if leaseUntil > now.Unix() {
			return ErrDeadLetterLeaseHeld
		}
		return ErrDeadLetterNotFound
	}
	return nil
}

// LoadForReplay decrypts the delivery of a leased row. The caller must have
// successfully claimed the lease with the same token.
func (s *deadLetterStore) LoadForReplay(ctx context.Context, id string) (DeadLetterDelivery, string, error) {
	var encrypted, event string
	err := s.db.QueryRowContext(ctx,
		`SELECT encrypted_delivery, event FROM webhook_dead_letters WHERE id = $1`,
		id,
	).Scan(&encrypted, &event)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeadLetterDelivery{}, "", ErrDeadLetterNotFound
		}
		return DeadLetterDelivery{}, "", err
	}
	plain, err := s.cipher.Decrypt(encrypted)
	if err != nil {
		return DeadLetterDelivery{}, "", fmt.Errorf("decrypt dead letter: %w", err)
	}
	var delivery DeadLetterDelivery
	if err := json.Unmarshal([]byte(plain), &delivery); err != nil {
		return DeadLetterDelivery{}, "", fmt.Errorf("unmarshal dead letter: %w", err)
	}
	return delivery, event, nil
}

// ReleaseLease clears the lease only when it still matches the given token,
// so a concurrent reclaim cannot be erased by a stale holder.
func (s *deadLetterStore) ReleaseLease(ctx context.Context, id, token string, now time.Time) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE webhook_dead_letters
        SET lease_token = '', lease_until = 0
        WHERE id = $1 AND lease_token = $2`,
		id, token,
	)
	return err
}

// DeleteLeased deletes the row only while our lease token still matches.
func (s *deadLetterStore) DeleteLeased(ctx context.Context, id, token string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM webhook_dead_letters WHERE id = $1 AND lease_token = $2`,
		id, token,
	)
	return err
}

// RunWebhookDeadLettersMigrationsForTest exposes the DDL to other packages'
// tests (admin handler). Test-only.
func RunWebhookDeadLettersMigrationsForTest(db *sql.DB) error {
	return runWebhookDeadLettersMigrations(db)
}

// SetTestHTTPClient swaps the HTTP client (tests use a vanilla client because
// the SSRF-safe dialer blocks httptest's loopback listener). Test-only.
func (ws *WebhookSender) SetTestHTTPClient(c *http.Client) {
	ws.httpClient = c
}
