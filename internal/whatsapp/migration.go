package whatsapp

import (
	"context"
	"database/sql"
)

func runMigrations(db *sql.DB) error {
	if err := runDeviceWebhooksMigrations(db); err != nil {
		return err
	}
	if err := runMessageJobsMigrations(db); err != nil {
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

func runMessageJobsMigrations(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS message_jobs (
        job_id TEXT PRIMARY KEY,
        phone_number TEXT NOT NULL,
        status TEXT NOT NULL,
        message_id TEXT,
        error_message TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        completed_at TIMESTAMP
    );

    CREATE INDEX IF NOT EXISTS idx_message_jobs_phone_status
        ON message_jobs(phone_number, status);

    CREATE INDEX IF NOT EXISTS idx_message_jobs_created_at
        ON message_jobs(created_at);`
	_, err := db.ExecContext(context.Background(), query)
	return err
}
