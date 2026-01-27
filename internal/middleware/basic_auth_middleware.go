package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
)

func (m *AuthMiddleware) BasicAuthMiddleware(expectedUser, expectedPassword string) fiber.Handler {
	return basicauth.New(basicauth.Config{
		Users: map[string]string{
			expectedUser: expectedPassword,
		},
		Realm: "Restricted",
	})
}
