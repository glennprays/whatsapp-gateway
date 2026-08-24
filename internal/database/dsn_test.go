package database

import (
	"net/url"
	"strings"
	"testing"
)

// TestNormalizePostgresDSN_URLForm covers the production DSN shape
// (postgresql://...?sslmode=disable): connect_timeout is injected, but
// keepalives_* must NOT be — lib/pq forwards unknown URL query params to the
// server as startup runtime parameters, which reject with 42704 (observed as a
// crash loop in prod). The %40-encoded password must survive.
func TestNormalizePostgresDSN_URLForm(t *testing.T) {
	got := normalizePostgresDSN("postgresql://waga_user:sup%40secret@10.10.10.3:58379/whatsapp_gateway?sslmode=disable")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("result unparseable: %v (%q)", err, got)
	}
	if u.User.Username() != "waga_user" || u.Host != "10.10.10.3:58379" || u.Path != "/whatsapp_gateway" {
		t.Fatalf("connection fields mangled: %q", got)
	}
	q := u.Query()
	want := map[string]string{
		"sslmode":         "disable",
		"connect_timeout": "10",
	}
	for k, v := range want {
		if q.Get(k) != v {
			t.Errorf("%s = %q, want %q (dsn %q)", k, q.Get(k), v, got)
		}
	}
	for _, banned := range []string{"keepalives", "keepalives_idle", "keepalives_interval", "keepalives_count"} {
		if q.Get(banned) != "" {
			t.Errorf("%s leaked into URL-form DSN: %q", banned, got)
		}
	}
}

// TestNormalizePostgresDSN_KeywordForm covers the lib/pq keyword/value DSN form.
func TestNormalizePostgresDSN_KeywordForm(t *testing.T) {
	got := normalizePostgresDSN("host=db user=waga dbname=gw sslmode=disable")
	for _, want := range []string{
		"host=db", "user=waga", "dbname=gw", "sslmode=disable",
		"connect_timeout=10", "keepalives_idle=30", "keepalives_interval=10", "keepalives_count=5",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}

	// Existing values are never overridden.
	got = normalizePostgresDSN("host=db connect_timeout=3 keepalives_idle=60")
	if !strings.Contains(got, "connect_timeout=3") || !strings.Contains(got, "keepalives_idle=60") {
		t.Errorf("existing params overwritten: %q", got)
	}
	if strings.Contains(got, "connect_timeout=10") || strings.Contains(got, "keepalives_idle=30") {
		t.Errorf("default leaked past explicit value: %q", got)
	}
}

// TestNormalizePostgresDSN_UnparseableURLPassthrough pins that a DSN the URL
// parser rejects is returned untouched so the driver reports the real error.
func TestNormalizePostgresDSN_UnparseableURLPassthrough(t *testing.T) {
	bad := "postgresql://%zz@host/db"
	if got := normalizePostgresDSN(bad); got != bad {
		t.Errorf("unparseable DSN modified: got %q", got)
	}
}
