package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/gofiber/fiber/v2"
)

// stubManager embeds the Manager interface so only the two methods the admin
// handler calls need implementing; any other call would panic (and shouldn't
// happen in these tests).
type stubManager struct {
	whatsapp.Manager
	inv *waDomain.SessionInventory
	one *waDomain.SessionInventoryItem
	err error
}

func (s stubManager) SessionInventory(ctx context.Context, traceID string) (*waDomain.SessionInventory, error) {
	return s.inv, s.err
}
func (s stubManager) GetOneSession(ctx context.Context, traceID, phone string) (*waDomain.SessionInventoryItem, error) {
	return s.one, s.err
}

func testLogger(t *testing.T) *customLog.Logger {
	t.Helper()
	l, err := customLog.New(customLog.Config{Service: "test", Env: "development", Level: customLog.InfoLevel, Output: customLog.OutputStdout})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func newApp(h *AdminHandler) *fiber.App {
	app := fiber.New()
	app.Get("/admin/sessions", h.Sessions)
	app.Get("/admin/sessions/:phone", h.Session)
	return app
}

func TestSessionsReturnsInventory(t *testing.T) {
	inv := &waDomain.SessionInventory{Instance: "gw-1", Count: 1, Sessions: []waDomain.SessionInventoryItem{{PhoneMasked: "62800xxxx", State: "connected"}}}
	h := NewAdminHandler(stubManager{inv: inv}, testLogger(t))
	app := newApp(h)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/admin/sessions", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got waDomain.SessionInventory
	json.NewDecoder(resp.Body).Decode(&got)
	if got.Count != 1 || len(got.Sessions) != 1 {
		t.Fatalf("unexpected inventory: %+v", got)
	}
}

func TestSessionNotFound(t *testing.T) {
	h := NewAdminHandler(stubManager{one: nil}, testLogger(t))
	app := newApp(h)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/admin/sessions/628999", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestSessionFound(t *testing.T) {
	item := &waDomain.SessionInventoryItem{PhoneMasked: "62800xxxx", State: "banned"}
	h := NewAdminHandler(stubManager{one: item}, testLogger(t))
	app := newApp(h)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/admin/sessions/628001", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}
