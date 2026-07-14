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
	if err := runIdempotencyKeysMigrations(db); err != nil {
		return err
	}
	if err := runSessionStatusMigrations(db); err != nil {
		return err
	}
	return nil
}

// runSessionStatusMigrations creates the table that records per-account session
// lifecycle state derived from whatsmeow events. Deliberately NO foreign key to
// whatsmeow_device: whatsmeow deletes the device row on logout, but a
// logged_out/banned record must survive so operators can still see why a
// session is gone. Dialect-neutral DDL (SQLite + Postgres).
func runSessionStatusMigrations(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS session_status (
        phone_number   TEXT PRIMARY KEY,
        state          TEXT NOT NULL,
        reason         TEXT,
        ban_expires_at TIMESTAMP,
        updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
	_, err := db.ExecContext(context.Background(), query)
	return err
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

// runIdempotencyKeysMigrations creates the table backing send idempotency. The
// (phone_number, idempotency_key) primary key is the dedup constraint the
// Idempotency-Key middleware relies on (INSERT ... ON CONFLICT DO NOTHING). DDL
// is dialect-neutral (works on SQLite + Postgres).
func runIdempotencyKeysMigrations(db *sql.DB) error {
	query := `
    CREATE TABLE IF NOT EXISTS idempotency_keys (
        phone_number TEXT NOT NULL,
        idempotency_key TEXT NOT NULL,
        request_hash TEXT NOT NULL,
        status TEXT NOT NULL,
        http_status INTEGER,
        response_body TEXT,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (phone_number, idempotency_key)
    );

    CREATE INDEX IF NOT EXISTS idx_idempotency_keys_created_at
        ON idempotency_keys(created_at);`
	_, err := db.ExecContext(context.Background(), query)
	return err
}
