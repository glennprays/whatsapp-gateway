package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"
)

// NewAdminAuth returns a bearer-token guard for the operator-only admin plane.
// It is only ever mounted when the secret is non-empty (the router keeps the
// whole plane unregistered when the secret is unset, so an unconfigured plane
// returns Fiber's default 404 rather than a 401 that would confirm it exists).
// The comparison is constant-time against the configured secret.
func NewAdminAuth(secret string) fiber.Handler {
	want := []byte("Bearer " + secret)
	return func(c *fiber.Ctx) error {
		got := []byte(c.Get("Authorization"))
		if subtle.ConstantTimeCompare(got, want) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Next()
	}
}
