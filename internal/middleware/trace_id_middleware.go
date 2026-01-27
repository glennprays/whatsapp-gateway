package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/glennprays/log"
	"github.com/google/uuid"
)

const (
	TraceIDHeader     = "X-Trace-ID"
	TraceIDContextKey = "trace_id"
)

// NewTraceIDMiddleware creates middleware that handles trace ID extraction/generation
func NewTraceIDMiddleware(logger *log.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Try to get trace ID from request header
		traceID := c.Get(TraceIDHeader)

		// Validate if it's a valid UUID
		if traceID != "" {
			if _, err := uuid.Parse(traceID); err != nil {
				// Invalid UUID format, generate new one and log warning
				newTraceID := uuid.New().String()
				logger.Warn(newTraceID, "Invalid trace ID format received, generating new one", []log.Field{
					log.String("invalid_trace_id", traceID),
					log.String("new_trace_id", newTraceID),
				})
				traceID = newTraceID
			}
		} else {
			// No trace ID provided, generate new one
			traceID = uuid.New().String()
		}

		// Store trace ID in context for use by handlers
		c.Locals(TraceIDContextKey, traceID)

		// Set trace ID in response header
		c.Set(TraceIDHeader, traceID)

		return c.Next()
	}
}

// GetTraceID extracts trace ID from fiber context
func GetTraceID(c *fiber.Ctx) string {
	traceID := c.Locals(TraceIDContextKey)
	if traceID != nil {
		if id, ok := traceID.(string); ok {
			return id
		}
	}
	// Fallback: generate new UUID if not found (shouldn't happen if middleware is applied)
	return uuid.New().String()
}
