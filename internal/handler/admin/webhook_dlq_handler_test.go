package admin

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	"github.com/gofiber/fiber/v2"
	_ "modernc.org/sqlite"
)

// newDLQTestApp builds a sender over an in-memory dead-letter DB plus a Fiber
// app exposing the DLQ routes behind the admin bearer, mirroring SetupRouter's
// registration (dark when ADMIN_API_SECRET is unset).
func newDLQTestApp(t *testing.T, secret string) (*fiber.App, *whatsapp.WebhookSender, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := whatsapp.RunWebhookDeadLettersMigrationsForTest(db); err != nil {
		t.Fatal(err)
	}
	cipher := cipherx.NewCipher("0123456789abcdef0123456789abcdef")
	sender := whatsapp.NewWebhookSender(cipher, testLogger(t), &config.Config{}, db)
	sender.SetTestHTTPClient(&http.Client{Timeout: 2 * time.Second})

	app := fiber.New()
	if secret != "" {
		h := NewWebhookDLQHandler(sender, testLogger(t))
		grp := app.Group("/admin", func(c *fiber.Ctx) error {
			auth := c.Get("Authorization")
			if auth != "Bearer "+secret {
				return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
			}
			return c.Next()
		})
		grp.Get("/webhook-dlq", h.List)
		grp.Post("/webhook-dlq/:id/replay", h.Replay)
	}
	return app, sender, db
}

// TestWebhookDLQDisabledWhenAdminSecretUnset: no secret => route never
// registered => 404 (dark plane).
func TestWebhookDLQDisabledWhenAdminSecretUnset(t *testing.T) {
	app, _, _ := newDLQTestApp(t, "")
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/admin/webhook-dlq", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (dark admin plane)", resp.StatusCode)
	}
}

// TestWebhookDLQRejectsInvalidBearer.
func TestWebhookDLQRejectsInvalidBearer(t *testing.T) {
	app, _, _ := newDLQTestApp(t, "sekrit")
	req := httptest.NewRequest(http.MethodGet, "/admin/webhook-dlq", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestWebhookDLQListValidatesPagination.
func TestWebhookDLQListValidatesPagination(t *testing.T) {
	app, _, _ := newDLQTestApp(t, "sekrit")
	get := func(q string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin/webhook-dlq"+q, nil)
		req.Header.Set("Authorization", "Bearer sekrit")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	for _, q := range []string{"?limit=0", "?limit=101", "?limit=abc", "?offset=-1", "?offset=x"} {
		if got := get(q); got != http.StatusBadRequest {
			t.Fatalf("q=%q status = %d, want 400", q, got)
		}
	}
	if got := get("?limit=100&offset=0"); got != http.StatusOK {
		t.Fatalf("valid pagination status = %d, want 200", got)
	}
}

// TestWebhookDLQReplayFlow: end-to-end list + replay through the HTTP
// surface — 404 for unknown id, 502 with row retained when the target 500s,
// 200 + delete when the target succeeds, and no sensitive fields in list.
func TestWebhookDLQReplayFlow(t *testing.T) {
	app, sender, db := newDLQTestApp(t, "sekrit")

	var failTarget = true
	var seenBody []byte
	var seenSig string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failTarget {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		seenBody = make([]byte, r.ContentLength)
		r.Body.Read(seenBody)
		seenSig = r.Header.Get("X-Webhook-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// Persist one dead letter by failing a delivery through SendAsync.
	enc, err := cipherx.NewCipher("0123456789abcdef0123456789abcdef").Encrypt("topsecret")
	if err != nil {
		t.Fatal(err)
	}
	sender.SendAsync(target.URL, enc, "message.sent", map[string]interface{}{"k": "v"})
	deadline := time.Now().Add(5 * time.Second)
	var n int
	for n == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		db.QueryRow("SELECT COUNT(*) FROM webhook_dead_letters").Scan(&n)
	}
	if n != 1 {
		t.Fatal("dead letter never persisted")
	}

	auth := func(req *http.Request) *http.Request {
		req.Header.Set("Authorization", "Bearer sekrit")
		return req
	}

	// 404 for a nonexistent id.
	resp, err := app.Test(auth(httptest.NewRequest(http.MethodPost, "/admin/webhook-dlq/nope/replay", nil)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing id status = %d, want 404", resp.StatusCode)
	}

	// List returns metadata only.
	resp, err = app.Test(auth(httptest.NewRequest(http.MethodGet, "/admin/webhook-dlq", nil)))
	if err != nil {
		t.Fatal(err)
	}
	var listResp struct {
		Items []whatsapp.DeadLetterItem `json:"items"`
		Limit int                       `json:"limit"`
	}
	json.NewDecoder(resp.Body).Decode(&listResp)
	if resp.StatusCode != http.StatusOK || len(listResp.Items) != 1 || listResp.Limit != 50 {
		t.Fatalf("list = %d items=%d limit=%d", resp.StatusCode, len(listResp.Items), listResp.Limit)
	}
	raw, _ := json.Marshal(listResp)
	for _, leak := range []string{target.URL, "topsecret", `"k"`} {
		if strings.Contains(string(raw), leak) {
			t.Fatalf("list leaked %q: %s", leak, raw)
		}
	}

	// Replay with a failing target: 502, row retained.
	id := listResp.Items[0].ID
	resp, err = app.Test(auth(httptest.NewRequest(http.MethodPost, "/admin/webhook-dlq/"+id+"/replay", nil)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("failing replay status = %d, want 502", resp.StatusCode)
	}
	db.QueryRow("SELECT COUNT(*) FROM webhook_dead_letters").Scan(&n)
	if n != 1 {
		t.Fatalf("row count after failed replay = %d, want 1", n)
	}

	// Retry immediately: the failed attempt released its lease, so this
	// reaches the target again; now it succeeds: 200, row deleted.
	failTarget = false
	resp, err = app.Test(auth(httptest.NewRequest(http.MethodPost, "/admin/webhook-dlq/"+id+"/replay", nil)))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("successful replay status = %d, want 200", resp.StatusCode)
	}
	var replayResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&replayResp)
	if replayResp["status"] != "delivered" {
		t.Fatalf("replay status body = %v", replayResp)
	}
	db.QueryRow("SELECT COUNT(*) FROM webhook_dead_letters").Scan(&n)
	if n != 0 {
		t.Fatalf("row count after successful replay = %d, want 0", n)
	}
	if !strings.Contains(string(seenBody), `"k":"v"`) {
		t.Fatalf("replayed payload lost: %s", seenBody)
	}
	if !strings.HasPrefix(seenSig, "sha256=") {
		t.Fatalf("replayed signature missing: %q", seenSig)
	}
}
