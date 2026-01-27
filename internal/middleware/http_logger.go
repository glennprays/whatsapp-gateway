package middleware

import (
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/gofiber/fiber/v2"
)

func NewHTTPLogger(log *log.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := GetTraceID(c)
		start := time.Now()

		// Process request
		err := c.Next()

		latency := time.Since(start)

		log.Info(requestID, "http request", map[string]any{
			"incoming_hostname": c.Hostname(),
			"status":            c.Response().StatusCode(),
			"method":            c.Method(),
			"path":              c.Path(),
			"ip":                utils.GetIPFromFiberCtx(c),
			"latency":           latency.String(),
		})

		return err
	}
}
