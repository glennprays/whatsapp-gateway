package whatsapp

import (
	"context"
	"database/sql"

	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

type whatsAppRepository struct {
	DB *sql.DB
}

type WhatsAppRepository interface {
	SetWebhook(ctx context.Context, jid string, webhookURL string, hmacSecret string) error
	GetWebhook(ctx context.Context, jid string) (*waDomain.Webhook, error)
	GetWebhookByPhone(ctx context.Context, phoneNumber string) (*waDomain.Webhook, string, error)
	DeleteWebhook(ctx context.Context, jid string) error
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
