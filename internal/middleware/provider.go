package middleware

import (
	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
	"github.com/gofiber/fiber/v2"
)

// ProvideAuthMiddleware initializes authentication middleware
func ProvideAuthMiddleware(jwtManager *auth.JWTManager) *AuthMiddleware {
	return NewAuthMiddleware(jwtManager)
}

// ProvideTraceIDMiddleware initializes trace ID middleware
func ProvideTraceIDMiddleware(logger *log.Logger) fiber.Handler {
	return NewTraceIDMiddleware(logger)
}
