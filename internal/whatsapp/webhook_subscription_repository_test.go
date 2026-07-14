package whatsapp

import (
	"context"
	"database/sql"
	"testing"

	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	_ "modernc.org/sqlite"
)

// newWebhookSubsTestDB spins up an in-memory SQLite with the webhook tables.
// A whatsmeow_device parent is created so the FK reference is satisfiable.
func newWebhookSubsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // single shared in-memory connection
	if _, err := db.Exec(`CREATE TABLE whatsmeow_device (jid TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := runDeviceWebhooksMigrations(db); err != nil {
		t.Fatal(err)
	}
	if err := runDeviceWebhookSubscriptionsMigrations(db, nil); err != nil {
		t.Fatal(err)
	}
	return db
}

func countSubs(t *testing.T, db *sql.DB, jid string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM device_webhook_subscriptions WHERE jid = ?", jid).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWebhookSubscriptionRoundTrip(t *testing.T) {
	db := newWebhookSubsTestDB(t)
	defer db.Close()
	repo := NewWhatsappRepository(db)
	ctx := context.Background()
	const jid = "628111@s.whatsapp.net"
	const urlA = "https://a.example/hook"
	const urlB = "https://b.example/hook"

	// Insert first subscription with a secret and an events filter.
	if err := repo.SetWebhookSubscription(ctx, jid, urlA, "ENC-A", "message.incoming"); err != nil {
		t.Fatalf("set A: %v", err)
	}
	// Upsert the SAME (jid,url): must update in place, not add a row.
	if err := repo.SetWebhookSubscription(ctx, jid, urlA, "ENC-A2", ""); err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	if n := countSubs(t, db, jid); n != 1 {
		t.Fatalf("after upsert of same (jid,url), want 1 row, got %d", n)
	}

	// Second distinct URL, no secret -> 2 rows.
	if err := repo.SetWebhookSubscription(ctx, jid, urlB, "", "message.sent"); err != nil {
		t.Fatalf("set B: %v", err)
	}
	subs, err := repo.GetWebhookSubscriptions(ctx, jid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("want 2 subscriptions, got %d", len(subs))
	}
	// Deterministic order (created_at, url) => A first.
	if subs[0].Url != urlA || subs[1].Url != urlB {
		t.Fatalf("unexpected order: %+v", subs)
	}
	// A: upsert overwrote hmac + events (events now '').
	if subs[0].HmacSecret != "ENC-A2" || subs[0].Events != "" {
		t.Fatalf("A not upserted correctly: %+v", subs[0])
	}
	// has_hmac derivation: A has a secret, B does not.
	if subs[0].HmacSecret == "" {
		t.Fatalf("A should have a secret")
	}
	if subs[1].HmacSecret != "" {
		t.Fatalf("B should have no secret, got %q", subs[1].HmacSecret)
	}

	// Delete one.
	if err := repo.DeleteWebhookSubscription(ctx, jid, urlA); err != nil {
		t.Fatalf("delete one: %v", err)
	}
	subs, _ = repo.GetWebhookSubscriptions(ctx, jid)
	if len(subs) != 1 || subs[0].Url != urlB {
		t.Fatalf("after delete-one, want only B, got %+v", subs)
	}

	// Delete all.
	if err := repo.DeleteAllWebhookSubscriptions(ctx, jid); err != nil {
		t.Fatalf("delete all: %v", err)
	}
	if n := countSubs(t, db, jid); n != 0 {
		t.Fatalf("after delete-all, want 0 rows, got %d", n)
	}
}

// TestWebhookSubscriptionMigrationPreservesData proves the backfill copies each
// legacy device_webhooks row into exactly one subscription (events=” = all)
// and is idempotent across repeated runs.
func TestWebhookSubscriptionMigrationPreservesData(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE whatsmeow_device (jid TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := runDeviceWebhooksMigrations(db); err != nil {
		t.Fatal(err)
	}

	const jid = "628222@s.whatsapp.net"
	if _, err := db.Exec(`INSERT INTO whatsmeow_device (jid) VALUES (?)`, jid); err != nil {
		t.Fatal(err)
	}
	repo := NewWhatsappRepository(db)
	if err := repo.SetWebhook(context.Background(), jid, "https://legacy.example/hook", "ENC-LEGACY"); err != nil {
		t.Fatal(err)
	}

	// First migration: backfill.
	if err := runDeviceWebhookSubscriptionsMigrations(db, nil); err != nil {
		t.Fatalf("migration 1: %v", err)
	}
	// Second migration: must be a no-op (idempotent).
	if err := runDeviceWebhookSubscriptionsMigrations(db, nil); err != nil {
		t.Fatalf("migration 2: %v", err)
	}

	subs, err := repo.GetWebhookSubscriptions(context.Background(), jid)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("want exactly 1 backfilled subscription, got %d", len(subs))
	}
	got := subs[0]
	if got.Url != "https://legacy.example/hook" || got.HmacSecret != "ENC-LEGACY" || got.Events != "" {
		t.Fatalf("backfill did not preserve data: %+v", got)
	}
}

// TestWebhookSubscriptionSeedRunsOnce proves the one-time backfill does NOT
// resurrect a subscription the operator deleted after upgrade, even when the
// delete leaves the subscriptions table globally empty and the process
// restarts (re-runs migrations).
func TestWebhookSubscriptionSeedRunsOnce(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE whatsmeow_device (jid TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := runDeviceWebhooksMigrations(db); err != nil {
		t.Fatal(err)
	}
	const jid = "628333@s.whatsapp.net"
	if _, err := db.Exec(`INSERT INTO whatsmeow_device (jid) VALUES (?)`, jid); err != nil {
		t.Fatal(err)
	}
	repo := NewWhatsappRepository(db)
	if err := repo.SetWebhook(context.Background(), jid, "https://old.example/hook", "ENC"); err != nil {
		t.Fatal(err)
	}

	// Upgrade: backfill seeds the subscription.
	if err := runDeviceWebhookSubscriptionsMigrations(db, nil); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if n := countSubs(t, db, jid); n != 1 {
		t.Fatalf("after seed want 1 subscription, got %d", n)
	}

	// Operator deletes it (empties the subscriptions table; device_webhooks is
	// frozen and never pruned).
	if err := repo.DeleteAllWebhookSubscriptions(context.Background(), jid); err != nil {
		t.Fatal(err)
	}

	// Restart re-runs migrations: the marker must keep the seed from resurrecting
	// the deleted subscription.
	if err := runDeviceWebhookSubscriptionsMigrations(db, nil); err != nil {
		t.Fatalf("re-run: %v", err)
	}
	if n := countSubs(t, db, jid); n != 0 {
		t.Fatalf("deleted subscription was resurrected: want 0, got %d", n)
	}
}

// TestWebhookSubscriptionSurvivesDeviceDelete proves the subscriptions table has
// no FK to whatsmeow_device: deleting the device row (as whatsmeow does on
// logout) must NOT cascade the subscription away, so the session.logged_out
// webhook dispatch can still read it. Foreign keys are enforced here so the
// legacy device_webhooks FK cascade actually fires.
func TestWebhookSubscriptionSurvivesDeviceDelete(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE whatsmeow_device (jid TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := runDeviceWebhooksMigrations(db); err != nil {
		t.Fatal(err)
	}
	if err := runDeviceWebhookSubscriptionsMigrations(db, nil); err != nil {
		t.Fatal(err)
	}
	const jid = "628444@s.whatsapp.net"
	if _, err := db.Exec(`INSERT INTO whatsmeow_device (jid) VALUES (?)`, jid); err != nil {
		t.Fatal(err)
	}
	repo := NewWhatsappRepository(db)
	if err := repo.SetWebhookSubscription(context.Background(), jid, "https://a.example/hook", "", ""); err != nil {
		t.Fatal(err)
	}

	// whatsmeow deletes the device row on logout.
	if _, err := db.Exec(`DELETE FROM whatsmeow_device WHERE jid = ?`, jid); err != nil {
		t.Fatal(err)
	}
	if n := countSubs(t, db, jid); n != 1 {
		t.Fatalf("subscription must survive device deletion (no cascade), got %d rows", n)
	}
}

// TestWebhookSubscriptionMigrationNormalizesLegacySecret proves the backfill
// decrypts legacy secrets so a no-secret webhook (stored as Encrypt("")) becomes
// an empty column (has_hmac=false), while a real secret is preserved.
func TestWebhookSubscriptionMigrationNormalizesLegacySecret(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE whatsmeow_device (jid TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := runDeviceWebhooksMigrations(db); err != nil {
		t.Fatal(err)
	}

	cipher := cipherx.NewCipher("0123456789abcdef0123456789abcdef") // 32 bytes
	encEmpty, err := cipher.Encrypt("")
	if err != nil {
		t.Fatal(err)
	}
	encReal, err := cipher.Encrypt("s3cr3t")
	if err != nil {
		t.Fatal(err)
	}

	repo := NewWhatsappRepository(db)
	const noSecretJID = "628555@s.whatsapp.net"
	const secretJID = "628666@s.whatsapp.net"
	for _, jid := range []string{noSecretJID, secretJID} {
		if _, err := db.Exec(`INSERT INTO whatsmeow_device (jid) VALUES (?)`, jid); err != nil {
			t.Fatal(err)
		}
	}
	// The legacy manager encrypted unconditionally, so even a no-secret webhook
	// has a non-empty ciphertext at rest.
	if err := repo.SetWebhook(context.Background(), noSecretJID, "https://n.example/hook", encEmpty); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetWebhook(context.Background(), secretJID, "https://s.example/hook", encReal); err != nil {
		t.Fatal(err)
	}

	if err := runDeviceWebhookSubscriptionsMigrations(db, cipher); err != nil {
		t.Fatalf("migration: %v", err)
	}

	noSecretSubs, _ := repo.GetWebhookSubscriptions(context.Background(), noSecretJID)
	if len(noSecretSubs) != 1 || noSecretSubs[0].HmacSecret != "" {
		t.Fatalf("legacy no-secret webhook must normalize to empty (has_hmac=false), got %+v", noSecretSubs)
	}
	secretSubs, _ := repo.GetWebhookSubscriptions(context.Background(), secretJID)
	if len(secretSubs) != 1 || secretSubs[0].HmacSecret != encReal {
		t.Fatalf("legacy real secret must be preserved verbatim, got %+v", secretSubs)
	}
}
