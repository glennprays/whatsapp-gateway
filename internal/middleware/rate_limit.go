package middleware

import (
	"fmt"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
	"github.com/gofiber/fiber/v2"
)

// NewRegisterRateLimiter returns a Fiber middleware that caps POST /register
// per client IP. An in-process memory limiter is intentional here: registration
// is infrequent and we want protection even without Redis. The limiter rejects
// (429) over-budget calls rather than queueing.
func NewRegisterRateLimiter(cfg *config.Config) fiber.Handler {
	if !cfg.RegisterRateLimitEnabled {
		return func(c *fiber.Ctx) error { return c.Next() }
	}

	window := time.Duration(cfg.RegisterRateLimitDurationSeconds) * time.Second
	if window <= 0 {
		window = time.Minute
	}
	limit := cfg.RegisterRateLimitRequests
	if limit <= 0 {
		limit = 5
	}

	limiter := ratelimiter.NewMemoryLimiter(ratelimiter.Config{
		Limit:  limit,
		Window: window,
		Prefix: "whatsapp:gateway:register:",
	})

	return func(c *fiber.Ctx) error {
		res, err := limiter.Allow(c.Context(), c.IP())
		if err != nil {
			// Fail open: a limiter error must not block registration.
			return c.Next()
		}
		if !res.Allowed {
			c.Set("Retry-After", fmt.Sprintf("%d", int(res.RetryAfter.Seconds())+1))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many registration requests, please retry later",
			})
		}
		return c.Next()
	}
}
