package whatsapp

import (
	"context"
	"database/sql"
	"testing"
	"time"

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
