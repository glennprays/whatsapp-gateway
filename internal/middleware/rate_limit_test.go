package middleware

import (
	"net/http"
	"testing"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/gofiber/fiber/v2"
)

func newTestApp(t *testing.T, cfg *config.Config) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Post("/register", NewRegisterRateLimiter(cfg), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})
	return app
}

func TestRegisterRateLimiter_BlocksAfterLimit(t *testing.T) {
	cfg := &config.Config{
		RegisterRateLimitEnabled:         true,
		RegisterRateLimitRequests:        2,
		RegisterRateLimitDurationSeconds: 60,
	}
	app := newTestApp(t, cfg)

	// First two from the same IP succeed.
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodPost, "http://example.com/register", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("req %d: %v", i+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("req %d: expected 200, got %d", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Third is rate-limited.
	req, _ := http.NewRequest(http.MethodPost, "http://example.com/register", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("over-limit req: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
}

func TestRegisterRateLimiter_Disabled(t *testing.T) {
	cfg := &config.Config{RegisterRateLimitEnabled: false, RegisterRateLimitRequests: 1}
	app := newTestApp(t, cfg)
	for i := 0; i < 5; i++ {
		req, _ := http.NewRequest(http.MethodPost, "http://example.com/register", nil)
		req.RemoteAddr = "9.9.9.9:1"
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("req %d: %v", i+1, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Errorf("disabled limiter req %d: expected 200, got %d", i+1, resp.StatusCode)
		}
		resp.Body.Close()
	}
}
