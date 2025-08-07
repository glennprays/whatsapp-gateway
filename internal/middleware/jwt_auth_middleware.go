package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/contextkeys"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
)

func (m *AuthMiddleware) JWTAuthentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, err := m.extractBearerToken(c)
		if err != nil {
			apiErr := httperror.FromError(err)
			c.AbortWithStatusJSON(apiErr.Status, apiErr)
			return
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
			c.AbortWithStatusJSON(apiErr.Status, apiErr)
			return
		}

		c.Set(string(contextkeys.PhoneNumber), claims.PhoneNumber)

		c.Next()
	}
}

func (m *AuthMiddleware) extractBearerToken(c *gin.Context) (string, error) {
	authHeader := c.GetHeader(AuthorizationHeaderKey)
	if authHeader == "" {
		appErr := errors.New("authorization header required")
		return "", errDomain.NewError(errDomain.ErrUnauthorized, appErr)
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || !strings.EqualFold(parts[0], AuthorizationTypeBearer) {
		appErr := errors.New("invalid authorization header format (must be Bearer token)")
		return "", errDomain.NewError(errDomain.ErrUnauthorized, appErr)
	}

	return parts[1], nil
}
