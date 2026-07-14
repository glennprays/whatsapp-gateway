package whatsapp

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	_ "modernc.org/sqlite"
)

func newSessionStatusTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1) // single shared in-memory connection
	if err := runSessionStatusMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestClientBanState(t *testing.T) {
	db := newSessionStatusTestDB(t)
	defer db.Close()
	repo := NewWhatsappRepository(db)
	ctx := context.Background()
	c := &client{cfg: &config.Config{OutboundPaceBanDefaultHoldSeconds: 3600}, repository: repo}

	// No row => not banned.
	if _, banned := c.BanState("111"); banned {
		t.Fatal("unknown account must not be banned")
	}

	// Connected => not banned.
	_ = repo.UpsertSessionStatus(ctx, "222", "connected", "", nil)
	if _, banned := c.BanState("222"); banned {
		t.Fatal("connected account must not be banned")
	}

	// Banned with a future expiry => banned until that time.
	future := time.Now().Add(30 * time.Minute)
	_ = repo.UpsertSessionStatus(ctx, "333", "banned", "too many", &future)
	until, banned := c.BanState("333")
	if !banned || until.Before(time.Now()) {
		t.Fatalf("account with future ban expiry must be banned, got until=%v banned=%v", until, banned)
	}

	// Banned but expiry already passed => not banned.
	past := time.Now().Add(-time.Minute)
	_ = repo.UpsertSessionStatus(ctx, "444", "banned", "old", &past)
	if _, banned := c.BanState("444"); banned {
		t.Fatal("account whose ban expiry has passed must not be banned")
	}

	// Banned with unknown duration (nil expiry) => held for the default hold
	// from updated_at (just now), so still banned.
	_ = repo.UpsertSessionStatus(ctx, "555", "banned", "unknown duration", nil)
	if _, banned := c.BanState("555"); !banned {
		t.Fatal("banned account with unknown duration must be held for the default hold")
	}
}

func TestSessionStatusUpsertAndList(t *testing.T) {
	db := newSessionStatusTestDB(t)
	defer db.Close()
	repo := NewWhatsappRepository(db)
	ctx := context.Background()

	// Insert.
	if err := repo.UpsertSessionStatus(ctx, "628111", "logged_out", "stream_error", nil); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Update via ON CONFLICT (same phone -> banned with expiry).
	exp := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := repo.UpsertSessionStatus(ctx, "628111", "banned", "temp ban", &exp); err != nil {
		t.Fatalf("update: %v", err)
	}

	statuses, err := repo.ListSessionStatuses(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("want 1 row after upsert, got %d", len(statuses))
	}
	got := statuses["628111"]
	if got.State != "banned" || got.Reason != "temp ban" {
		t.Fatalf("upsert did not overwrite: %+v", got)
	}
	if got.BanExpiresAt == nil || !got.BanExpiresAt.Equal(exp) {
		t.Fatalf("ban_expires_at not persisted: %+v", got.BanExpiresAt)
	}
}

// TestSessionStatusSurvivesDeviceDelete proves the missing FK: deleting the
// whatsmeow_device row must NOT cascade to session_status (the whole point of
// recording status is to outlive the device row on logout).
func TestSessionStatusSurvivesDeviceDelete(t *testing.T) {
	db := newSessionStatusTestDB(t)
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE whatsmeow_device (jid TEXT PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO whatsmeow_device (jid) VALUES ('628111@s.whatsapp.net')`); err != nil {
		t.Fatal(err)
	}
	repo := NewWhatsappRepository(db)
	ctx := context.Background()
	if err := repo.UpsertSessionStatus(ctx, "628111", "logged_out", "stream_error", nil); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Exec(`DELETE FROM whatsmeow_device`); err != nil {
		t.Fatal(err)
	}

	statuses, err := repo.ListSessionStatuses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := statuses["628111"]; !ok {
		t.Fatal("session_status row was removed when device row was deleted; unexpected FK cascade")
	}
}
