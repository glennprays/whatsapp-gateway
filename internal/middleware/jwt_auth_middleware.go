package middleware

import (
	"errors"
	"strings"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/contextkeys"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
	"github.com/gofiber/fiber/v2"
)

func (m *AuthMiddleware) JWTAuthentication() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tokenString, err := m.extractBearerToken(c)
		if err != nil {
			apiErr := httperror.FromError(err)
			return c.Status(apiErr.Status).JSON(apiErr)
		}

		claims, err := m.JwtManager.ValidateToken(tokenString)
		if err != nil {
			var (
				message = "Invalid token"
				svcErr  = errDomain.ErrUnauthorized
			)

			switch {
			case errors.Is(err, auth.ErrTokenExpired):
				message = "Token has expired"
			case errors.Is(err, auth.ErrTokenInvalid), errors.Is(err, auth.ErrMissingClaims):
				message = "Invalid token"
			default:
				message = "Internal error during token validation"
			}

			appErr := errDomain.NewError(svcErr, errors.New(message))
			apiErr := httperror.FromError(appErr)
			return c.Status(apiErr.Status).JSON(apiErr)
		}

		c.Locals(string(contextkeys.PhoneNumber), claims.PhoneNumber)

		return c.Next()
	}
}

func (m *AuthMiddleware) extractBearerToken(c *fiber.Ctx) (string, error) {
	authHeader := c.Get(AuthorizationHeaderKey)
	if authHeader == "" {
		appErr := errors.New("authorization required")
		return "", errDomain.NewError(errDomain.ErrUnauthorized, appErr)
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], AuthorizationTypeBearer) {
		appErr := errors.New("invalid authorization header format (must be Bearer token)")
		return "", errDomain.NewError(errDomain.ErrUnauthorized, appErr)
	}

	return parts[1], nil
}
