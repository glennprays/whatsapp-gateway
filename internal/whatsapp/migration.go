package whatsapp

import (
	"context"
	"database/sql"

	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
)

func runMigrations(db *sql.DB, cipher *cipherx.Cipher) error {
	if err := runDeviceWebhooksMigrations(db); err != nil {
		return err
	}
	if err := runDeviceWebhookSubscriptionsMigrations(db, cipher); err != nil {
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

// runDeviceWebhookSubscriptionsMigrations creates the multi-URL subscription
// table and seeds it, exactly once, from the legacy single-row device_webhooks
// table: each existing webhook becomes one subscription with an empty events
// filter (= all events), preserving today's "one URL receives everything".
//
// Deliberately NO foreign key to whatsmeow_device (mirroring session_status).
// whatsmeow deletes the device row on logout; an ON DELETE CASCADE here would
// race the session.logged_out webhook and cascade-wipe the very subscriptions
// its dispatch must read. Keeping the rows also lets webhook config survive a
// re-pair with the same JID.
//
// The backfill is gated by a persistent schema_seed_markers row, NOT by the
// target table being empty: an emptiness guard re-seeds (resurrects) a webhook
// the operator deleted after upgrade, every time the subscriptions table later
// goes empty (single-account / delete-all). The marker makes it run once, ever.
// Legacy secrets are decrypt-normalized so a no-secret webhook (stored as
// Encrypt("")) reports has_hmac=false like a freshly created one.
//
// Portable DDL (SQLite 3.24+/Postgres 9.5+): $N placeholders, no arrays.
func runDeviceWebhookSubscriptionsMigrations(db *sql.DB, cipher *cipherx.Cipher) error {
	ctx := context.Background()
	ddl := `
    CREATE TABLE IF NOT EXISTS device_webhook_subscriptions (
        jid         TEXT NOT NULL,
        url         TEXT NOT NULL,
        hmac_secret TEXT NOT NULL DEFAULT '',
        events      TEXT NOT NULL DEFAULT '',
        created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (jid, url)
    );

    CREATE TABLE IF NOT EXISTS schema_seed_markers (
        name       TEXT PRIMARY KEY,
        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );`
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		return err
	}

	// Run the one-time backfill only if the marker is absent.
	var seeded int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM schema_seed_markers WHERE name = 'device_webhooks_backfill'`).Scan(&seeded); err != nil {
		return err
	}
	if seeded > 0 {
		return nil
	}

	// Read all legacy rows first, then close the cursor before inserting: the
	// SQLite dev DB runs on a single connection, so an open cursor would
	// deadlock a concurrent INSERT.
	type legacyWebhook struct{ jid, url, secret string }
	rows, err := db.QueryContext(ctx, `SELECT jid, webhook_url, hmac_secret FROM device_webhooks`)
	if err != nil {
		return err
	}
	var pending []legacyWebhook
	for rows.Next() {
		var l legacyWebhook
		if err := rows.Scan(&l.jid, &l.url, &l.secret); err != nil {
			rows.Close()
			return err
		}
		// A legacy no-secret webhook was stored as Encrypt(""); normalize it to
		// "" so has_hmac is honest. A real secret keeps its ciphertext at rest.
		if cipher != nil && l.secret != "" {
			if plain, derr := cipher.Decrypt(l.secret); derr != nil || plain == "" {
				l.secret = ""
			}
		}
		pending = append(pending, l)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, l := range pending {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO device_webhook_subscriptions (jid, url, hmac_secret, events)
             VALUES ($1, $2, $3, '') ON CONFLICT (jid, url) DO NOTHING`,
			l.jid, l.url, l.secret); err != nil {
			return err
		}
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO schema_seed_markers (name) VALUES ('device_webhooks_backfill') ON CONFLICT (name) DO NOTHING`)
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
