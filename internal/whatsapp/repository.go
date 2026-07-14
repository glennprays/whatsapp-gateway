package whatsapp

import (
	"context"
	"database/sql"
	"time"

	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

type whatsAppRepository struct {
	DB *sql.DB
}

type WhatsAppRepository interface {
	// Legacy single-row webhook (kept for rollback; no longer used by dispatch).
	SetWebhook(ctx context.Context, jid string, webhookURL string, hmacSecret string) error
	GetWebhook(ctx context.Context, jid string) (*waDomain.Webhook, error)
	GetWebhookByPhone(ctx context.Context, phoneNumber string) (*waDomain.Webhook, string, error)
	DeleteWebhook(ctx context.Context, jid string) error

	// Multi-URL / per-event subscriptions.
	SetWebhookSubscription(ctx context.Context, jid, url, hmacSecret, events string) error
	GetWebhookSubscriptions(ctx context.Context, jid string) ([]waDomain.WebhookSubscription, error)
	DeleteWebhookSubscription(ctx context.Context, jid, url string) error
	DeleteAllWebhookSubscriptions(ctx context.Context, jid string) error

	UpsertSessionStatus(ctx context.Context, phone, state, reason string, banExpiresAt *time.Time) error
	ListSessionStatuses(ctx context.Context) (map[string]waDomain.SessionStatus, error)
	// GetSessionStatus is a single-row PK lookup (nil when absent) for the pacer
	// ban gate's hot path — cheaper than scanning the whole table per send.
	GetSessionStatus(ctx context.Context, phone string) (*waDomain.SessionStatus, error)
}

func NewWhatsappRepository(db *sql.DB) WhatsAppRepository {
	return &whatsAppRepository{DB: db}
}

func (r *whatsAppRepository) SetWebhook(ctx context.Context, jid string, webhookURL string, hmacSecret string) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO device_webhooks (jid, webhook_url, hmac_secret)
		VALUES ($1, $2, $3)
		ON CONFLICT (jid)
		DO UPDATE SET webhook_url = EXCLUDED.webhook_url, updated_at = CURRENT_TIMESTAMP
	`, jid, webhookURL, hmacSecret)
	return err
}

func (r *whatsAppRepository) GetWebhook(ctx context.Context, jid string) (*waDomain.Webhook, error) {
	var webhookURL sql.NullString
	var hmacSecret sql.NullString
	err := r.DB.QueryRowContext(ctx, "SELECT webhook_url, hmac_secret FROM device_webhooks WHERE jid = $1", jid).Scan(&webhookURL, &hmacSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	webhook := waDomain.Webhook{}
	if webhookURL.Valid {
		webhook.Url = webhookURL.String
	}
	if hmacSecret.Valid {
		webhook.HmacSecret = hmacSecret.String
	}
	return &webhook, nil
}

func (r *whatsAppRepository) GetWebhookByPhone(ctx context.Context, phoneNumber string) (*waDomain.Webhook, string, error) {
	var webhookURL sql.NullString
	var hmacSecret sql.NullString
	var jid string

	// Query webhook by joining with whatsmeow_device table
	// The JID format is: phoneNumber@s.whatsapp.net
	query := `
		SELECT dw.webhook_url, dw.hmac_secret, dw.jid
		FROM device_webhooks dw
		INNER JOIN whatsmeow_device wd ON dw.jid = wd.jid
		WHERE wd.jid LIKE $1
		LIMIT 1
	`

	err := r.DB.QueryRowContext(ctx, query, phoneNumber+"@%").Scan(&webhookURL, &hmacSecret, &jid)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", nil
		}
		return nil, "", err
	}

	webhook := waDomain.Webhook{}
	if webhookURL.Valid {
		webhook.Url = webhookURL.String
	}
	if hmacSecret.Valid {
		webhook.HmacSecret = hmacSecret.String
	}

	return &webhook, jid, nil
}

func (r *whatsAppRepository) DeleteWebhook(ctx context.Context, jid string) error {
	_, err := r.DB.ExecContext(ctx, "DELETE FROM device_webhooks WHERE jid = $1", jid)
	return err
}

// SetWebhookSubscription upserts one (jid, url) subscription. Re-posting the
// same URL edits its hmac/events filter. hmacSecret is stored encrypted (” =
// no secret); events is the comma-separated catalog subset (” = all events).
func (r *whatsAppRepository) SetWebhookSubscription(ctx context.Context, jid, url, hmacSecret, events string) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO device_webhook_subscriptions (jid, url, hmac_secret, events)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (jid, url)
		DO UPDATE SET hmac_secret = EXCLUDED.hmac_secret, events = EXCLUDED.events, updated_at = CURRENT_TIMESTAMP
	`, jid, url, hmacSecret, events)
	return err
}

// GetWebhookSubscriptions returns every subscription for a jid, oldest first
// (url tie-break) so the back-compat "first subscription" is deterministic.
func (r *whatsAppRepository) GetWebhookSubscriptions(ctx context.Context, jid string) ([]waDomain.WebhookSubscription, error) {
	rows, err := r.DB.QueryContext(ctx, `
		SELECT url, hmac_secret, events
		FROM device_webhook_subscriptions
		WHERE jid = $1
		ORDER BY created_at, url
	`, jid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []waDomain.WebhookSubscription
	for rows.Next() {
		var url string
		var hmacSecret sql.NullString
		var events sql.NullString
		if err := rows.Scan(&url, &hmacSecret, &events); err != nil {
			return nil, err
		}
		s := waDomain.WebhookSubscription{Url: url}
		if hmacSecret.Valid {
			s.HmacSecret = hmacSecret.String
		}
		if events.Valid {
			s.Events = events.String
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}

func (r *whatsAppRepository) DeleteWebhookSubscription(ctx context.Context, jid, url string) error {
	_, err := r.DB.ExecContext(ctx, "DELETE FROM device_webhook_subscriptions WHERE jid = $1 AND url = $2", jid, url)
	return err
}

func (r *whatsAppRepository) DeleteAllWebhookSubscriptions(ctx context.Context, jid string) error {
	_, err := r.DB.ExecContext(ctx, "DELETE FROM device_webhook_subscriptions WHERE jid = $1", jid)
	return err
}

// UpsertSessionStatus records the latest lifecycle state for an account. Keyed
// by bare phone number (repo convention: no normalization). Portable upsert
// (SQLite modernc + Postgres). banExpiresAt is nil except for temporary bans.
func (r *whatsAppRepository) UpsertSessionStatus(ctx context.Context, phone, state, reason string, banExpiresAt *time.Time) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO session_status (phone_number, state, reason, ban_expires_at, updated_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (phone_number)
		DO UPDATE SET state = EXCLUDED.state, reason = EXCLUDED.reason,
			ban_expires_at = EXCLUDED.ban_expires_at, updated_at = CURRENT_TIMESTAMP
	`, phone, state, reason, banExpiresAt)
	return err
}

// ListSessionStatuses returns every persisted status keyed by bare phone number.
func (r *whatsAppRepository) ListSessionStatuses(ctx context.Context) (map[string]waDomain.SessionStatus, error) {
	rows, err := r.DB.QueryContext(ctx, "SELECT phone_number, state, reason, ban_expires_at, updated_at FROM session_status")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]waDomain.SessionStatus)
	for rows.Next() {
		var phone string
		var state string
		var reason sql.NullString
		var banExpiresAt sql.NullTime
		var updatedAt sql.NullTime
		if err := rows.Scan(&phone, &state, &reason, &banExpiresAt, &updatedAt); err != nil {
			return nil, err
		}
		s := waDomain.SessionStatus{State: state}
		if reason.Valid {
			s.Reason = reason.String
		}
		if banExpiresAt.Valid {
			t := banExpiresAt.Time
			s.BanExpiresAt = &t
		}
		if updatedAt.Valid {
			s.UpdatedAt = updatedAt.Time
		}
		out[phone] = s
	}
	return out, rows.Err()
}

// GetSessionStatus returns one account's persisted status, or (nil, nil) when no
// row exists. Single-row PK lookup for the pacer ban gate.
func (r *whatsAppRepository) GetSessionStatus(ctx context.Context, phone string) (*waDomain.SessionStatus, error) {
	var state string
	var reason sql.NullString
	var banExpiresAt sql.NullTime
	var updatedAt sql.NullTime
	err := r.DB.QueryRowContext(ctx,
		"SELECT state, reason, ban_expires_at, updated_at FROM session_status WHERE phone_number = $1", phone,
	).Scan(&state, &reason, &banExpiresAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s := &waDomain.SessionStatus{State: state}
	if reason.Valid {
		s.Reason = reason.String
	}
	if banExpiresAt.Valid {
		t := banExpiresAt.Time
		s.BanExpiresAt = &t
	}
	if updatedAt.Valid {
		s.UpdatedAt = updatedAt.Time
	}
	return s, nil
}
