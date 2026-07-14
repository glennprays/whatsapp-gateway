package middleware

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/gofiber/fiber/v2"
)

// IdempotencyMiddleware dedupes mutating requests that carry an Idempotency-Key
// header. It is DB-backed (table idempotency_keys, keyed (phone_number, key)),
// so dedup survives restarts and is shared across instances. The check runs
// before the handler — hence before the queue/direct fork — so it covers both
// send modes. See ADR 0003.
//
// Guarantee: enqueued-once (queue mode) / sent-once (direct mode), NOT
// delivered-once — a replayed queue response is still 202+job_id.
type IdempotencyMiddleware struct {
	db             *sql.DB
	logger         *log.Logger
	ttl            time.Duration // how long a completed response stays replayable
	pendingTimeout time.Duration // after this, a pending row is assumed abandoned (crash) and can be taken over
}

type idempotencyRecord struct {
	requestHash  string
	status       string
	httpStatus   int
	responseBody []byte
	createdAt    time.Time
}

// NewIdempotencyMiddleware builds the middleware and starts a background sweeper
// that bounds table growth by deleting rows older than the TTL.
func NewIdempotencyMiddleware(db *sql.DB, cfg *config.Config, logger *log.Logger) *IdempotencyMiddleware {
	ttl := time.Duration(cfg.IdempotencyTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	pending := time.Duration(cfg.IdempotencyPendingTimeoutSeconds) * time.Second
	if pending <= 0 {
		pending = 30 * time.Second
	}
	m := &IdempotencyMiddleware{db: db, logger: logger, ttl: ttl, pendingTimeout: pending}
	go m.sweepLoop()
	return m
}

// Handler returns the Fiber middleware. It must be mounted AFTER JWT auth so the
// account phone number is on the context.
func (m *IdempotencyMiddleware) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		key := trimHeader(c.Get("Idempotency-Key"))
		// Opt-in: no key, or a non-mutating method, means normal processing.
		if key == "" || c.Method() == fiber.MethodGet {
			return c.Next()
		}
		phoneNumber, ok := utils.MustGetPhoneNumber(c)
		if !ok || phoneNumber == "" {
			return c.Next() // no identity yet; the auth layer will reject
		}

		hash := requestHash(c)
		ctx := c.Context()

		inserted, err := m.tryInsertPending(ctx, phoneNumber, key, hash)
		if err != nil {
			// Fail-open: a DB hiccup must not block a legitimate send.
			m.logger.Error("", "idempotency insert failed; proceeding without dedup",
				nil, log.Error(err))
			return c.Next()
		}

		if inserted {
			return m.runAndRecord(c, phoneNumber, key)
		}
		return m.replayOrReject(c, phoneNumber, key, hash)
	}
}

// runAndRecord is the winner path: run the handler, then snapshot a successful
// response for future replays (failures are not cached, so a retry can succeed).
func (m *IdempotencyMiddleware) runAndRecord(c *fiber.Ctx, phoneNumber, key string) error {
	if err := c.Next(); err != nil {
		m.delete(context.Background(), phoneNumber, key)
		return err
	}
	status := c.Response().StatusCode()
	if status >= 200 && status < 300 {
		// Copy the body — the fasthttp buffer is reused after the handler returns.
		body := append([]byte(nil), c.Response().Body()...)
		m.complete(context.Background(), phoneNumber, key, status, body)
	} else {
		m.delete(context.Background(), phoneNumber, key)
	}
	return nil
}

// replayOrReject is the loser path: a row already exists for (phone, key).
func (m *IdempotencyMiddleware) replayOrReject(c *fiber.Ctx, phoneNumber, key, hash string) error {
	rec, err := m.get(c.Context(), phoneNumber, key)
	if err != nil || rec == nil {
		return c.Next() // lost the row to a concurrent sweep/cleanup — just proceed
	}

	// Expired → treat as absent and let this request run fresh.
	if time.Since(rec.createdAt) > m.ttl {
		m.delete(c.Context(), phoneNumber, key)
		return c.Next()
	}

	// Same key, different request body — never silently re-send or return a stale result.
	if rec.requestHash != hash {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error": "Idempotency-Key was reused with a different request body",
		})
	}

	if rec.status == "pending" {
		// A crashed winner may leave a pending row; after pendingTimeout, take over.
		if time.Since(rec.createdAt) > m.pendingTimeout {
			m.delete(c.Context(), phoneNumber, key)
			return c.Next()
		}
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "a request with this Idempotency-Key is already in progress",
		})
	}

	// Completed → replay the stored response verbatim.
	c.Set("Idempotent-Replay", "true")
	c.Response().Header.SetContentType(fiber.MIMEApplicationJSON)
	return c.Status(rec.httpStatus).Send(rec.responseBody)
}

// tryInsertPending inserts a pending row. It returns true when this caller won
// the race (row inserted), false when a row already existed (ON CONFLICT). The
// $-placeholder + ON CONFLICT form works on both SQLite (modernc) and Postgres.
func (m *IdempotencyMiddleware) tryInsertPending(ctx context.Context, phoneNumber, key, hash string) (bool, error) {
	res, err := m.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (phone_number, idempotency_key, request_hash, status, created_at)
		VALUES ($1, $2, $3, 'pending', $4)
		ON CONFLICT (phone_number, idempotency_key) DO NOTHING
	`, phoneNumber, key, hash, time.Now())
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (m *IdempotencyMiddleware) get(ctx context.Context, phoneNumber, key string) (*idempotencyRecord, error) {
	var rec idempotencyRecord
	var httpStatus sql.NullInt64
	var body sql.NullString
	err := m.db.QueryRowContext(ctx, `
		SELECT request_hash, status, http_status, response_body, created_at
		FROM idempotency_keys
		WHERE phone_number = $1 AND idempotency_key = $2
	`, phoneNumber, key).Scan(&rec.requestHash, &rec.status, &httpStatus, &body, &rec.createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec.httpStatus = int(httpStatus.Int64)
	rec.responseBody = []byte(body.String)
	return &rec, nil
}

func (m *IdempotencyMiddleware) complete(ctx context.Context, phoneNumber, key string, httpStatus int, body []byte) {
	_, err := m.db.ExecContext(ctx, `
		UPDATE idempotency_keys
		SET status = 'completed', http_status = $1, response_body = $2
		WHERE phone_number = $3 AND idempotency_key = $4
	`, httpStatus, string(body), phoneNumber, key)
	if err != nil {
		m.logger.Error("", "idempotency complete update failed", nil, log.Error(err))
	}
}

func (m *IdempotencyMiddleware) delete(ctx context.Context, phoneNumber, key string) {
	if _, err := m.db.ExecContext(ctx, `
		DELETE FROM idempotency_keys WHERE phone_number = $1 AND idempotency_key = $2
	`, phoneNumber, key); err != nil {
		m.logger.Error("", "idempotency delete failed", nil, log.Error(err))
	}
}

// sweepLoop periodically deletes expired rows so keys that are never retried
// don't grow the table without bound (lazy delete-on-read only reclaims rows
// that are looked up again).
func (m *IdempotencyMiddleware) sweepLoop() {
	interval := min(m.ttl, time.Hour)
	interval = max(interval, time.Minute)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := m.db.ExecContext(ctx, `DELETE FROM idempotency_keys WHERE created_at < $1`, time.Now().Add(-m.ttl))
		cancel()
		if err != nil {
			// Harmless around shutdown (DB closing); log and keep going.
			m.logger.Warn("", "idempotency sweep failed", nil, log.Error(err))
		}
	}
}

func requestHash(c *fiber.Ctx) string {
	h := sha256.New()
	h.Write([]byte(c.Method()))
	h.Write([]byte{'\n'})
	h.Write([]byte(c.Path()))
	h.Write([]byte{'\n'})
	h.Write(c.Body())
	return hex.EncodeToString(h.Sum(nil))
}

func trimHeader(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}
