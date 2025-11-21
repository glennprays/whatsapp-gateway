package whatsapp

import (
	"context"
	"database/sql"
)

type whatsAppRepository struct {
	DB *sql.DB
}

type WhatsAppRepository interface {
	SetWebhook(ctx context.Context, jid string, webhookURL string) error
	GetWebhook(ctx context.Context, jid string) (*string, error)
	DeleteWebhook(ctx context.Context, jid string) error
}

func NewWhatsappRepository(db *sql.DB) WhatsAppRepository {
	return &whatsAppRepository{DB: db}
}

func (r *whatsAppRepository) SetWebhook(ctx context.Context, jid string, webhookURL string) error {
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO device_webhooks (jid, webhook_url)
		VALUES ($1, $2)
		ON CONFLICT (jid)
		DO UPDATE SET webhook_url = EXCLUDED.webhook_url, updated_at = CURRENT_TIMESTAMP
	`, jid, webhookURL)
	return err
}

func (r *whatsAppRepository) GetWebhook(ctx context.Context, jid string) (*string, error) {
	var webhookURL string
	err := r.DB.QueryRowContext(ctx, "SELECT webhook_url FROM device_webhooks WHERE jid = $1", jid).Scan(&webhookURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &webhookURL, nil
}

func (r *whatsAppRepository) DeleteWebhook(ctx context.Context, jid string) error {
	_, err := r.DB.ExecContext(ctx, "DELETE FROM device_webhooks WHERE jid = $1", jid)
	return err
}

func (r *whatsAppRepository) GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error) {
	return GetWebhookURL(ctx, phoneNumber)
}
