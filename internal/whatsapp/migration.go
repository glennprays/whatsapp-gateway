package whatsapp

import (
	"context"
	"database/sql"
)

func runMigrations(db *sql.DB) error {
	if err := runDeviceWebhooksMigrations(db); err != nil {
		return err
	}
	return nil
}

func runDeviceWebhooksMigrations(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS device_webhooks (
        jid TEXT PRIMARY KEY REFERENCES whatsmeow_device(jid) ON DELETE CASCADE,
        webhook_url TEXT NOT NULL,
				hmac_secret TEXT NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
	_, err := db.ExecContext(context.Background(), query)
	return err
}
