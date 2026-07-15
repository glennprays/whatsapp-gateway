package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func newAdminApp(secret string) *fiber.App {
	app := fiber.New()
	app.Get("/admin/ping", NewAdminAuth(secret), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})
	return app
}

func TestAdminAuth(t *testing.T) {
	const secret = "s3cr3t-token"
	app := newAdminApp(secret)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"valid bearer", "Bearer " + secret, http.StatusOK},
		{"wrong secret", "Bearer nope", http.StatusUnauthorized},
		{"missing header", "", http.StatusUnauthorized},
		{"raw secret without bearer", secret, http.StatusUnauthorized},
		{"lowercase scheme", "bearer " + secret, http.StatusUnauthorized},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}
