package whatsapp_handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/contextkeys"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/gofiber/fiber/v2"
)

// stubWebhookManager embeds whatsapp.Manager so only the webhook methods need
// implementing. It records delete routing so the handler's body-vs-no-body
// branch is observable.
type stubWebhookManager struct {
	whatsapp.Manager
	subs       []waDomain.WebhookSubscription
	deletedURL string
	deletedAll bool
}

func (s *stubWebhookManager) ListWebhookSubscriptions(ctx context.Context, traceID, phoneNumber string) ([]waDomain.WebhookSubscription, error) {
	return s.subs, nil
}
func (s *stubWebhookManager) SetWebhookSubscription(ctx context.Context, traceID, phoneNumber string, webhook *waDomain.Webhook) error {
	return nil
}
func (s *stubWebhookManager) DeleteWebhookSubscription(ctx context.Context, traceID, phoneNumber, url string) error {
	s.deletedURL = url
	return nil
}
func (s *stubWebhookManager) DeleteAllWebhookSubscriptions(ctx context.Context, traceID, phoneNumber string) error {
	s.deletedAll = true
	return nil
}

func newWebhookApp(t *testing.T, mgr whatsapp.Manager) *fiber.App {
	t.Helper()
	logger, err := customLog.New(customLog.Config{Service: "test", Env: "development", Level: customLog.ErrorLevel, Output: customLog.OutputStdout})
	if err != nil {
		t.Fatal(err)
	}
	uc := whatsapp_usecase.NewWhatsappWebhookUsecase(mgr, logger)
	h := NewWhatsappWebhookHandler(uc, logger)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals(string(contextkeys.PhoneNumber), "628111")
		return c.Next()
	})
	app.Get("/webhook", h.GetWebhookURL)
	app.Post("/webhook", h.SetWebhookURL)
	app.Delete("/webhook", h.DeleteWebhookURL)
	return app
}

func TestGetWebhookShapeAndNoHmacLeak(t *testing.T) {
	mgr := &stubWebhookManager{subs: []waDomain.WebhookSubscription{
		{Url: "https://first.example", HmacSecret: "ENC", Events: "message.incoming,message.sent"},
		{Url: "https://second.example", HmacSecret: "", Events: ""},
	}}
	app := newWebhookApp(t, mgr)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/webhook", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	raw := rawBody(t, resp)
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatal(err)
	}

	// Legacy top-level url = first subscription.
	if got := body["url"]; got != "https://first.example" {
		t.Fatalf("url = %v, want first subscription", got)
	}
	subs, ok := body["subscriptions"].([]interface{})
	if !ok || len(subs) != 2 {
		t.Fatalf("subscriptions = %v, want 2", body["subscriptions"])
	}
	first := subs[0].(map[string]interface{})
	if first["has_hmac"] != true {
		t.Fatalf("first has_hmac = %v, want true", first["has_hmac"])
	}
	events := first["events"].([]interface{})
	if len(events) != 2 {
		t.Fatalf("first events = %v, want 2", events)
	}
	second := subs[1].(map[string]interface{})
	if second["has_hmac"] != false {
		t.Fatalf("second has_hmac = %v, want false", second["has_hmac"])
	}
	if ev := second["events"].([]interface{}); len(ev) != 0 {
		t.Fatalf("second events = %v, want [] (all)", ev)
	}

	// The encrypted secret must never appear anywhere in the response.
	if strings.Contains(raw, "ENC") || strings.Contains(raw, "hmac_secret") {
		t.Fatalf("HMAC secret leaked in GET response: %s", raw)
	}
}

func TestDeleteWebhookRouting(t *testing.T) {
	// No body -> delete all.
	mgr := &stubWebhookManager{}
	app := newWebhookApp(t, mgr)
	resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/webhook", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !mgr.deletedAll || mgr.deletedURL != "" {
		t.Fatalf("no-body DELETE must delete all; deletedAll=%v deletedURL=%q", mgr.deletedAll, mgr.deletedURL)
	}

	// Body {"url":...} -> delete that one.
	mgr2 := &stubWebhookManager{}
	app2 := newWebhookApp(t, mgr2)
	req := httptest.NewRequest(http.MethodDelete, "/webhook", bytes.NewReader([]byte(`{"url":"https://one.example"}`)))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := app2.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp2.StatusCode)
	}
	if mgr2.deletedAll || mgr2.deletedURL != "https://one.example" {
		t.Fatalf("body DELETE must delete one; deletedAll=%v deletedURL=%q", mgr2.deletedAll, mgr2.deletedURL)
	}
}

func rawBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String()
}
