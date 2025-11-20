package middleware

import (
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
)

const (
	AuthorizationHeaderKey  = "Authorization"
	AuthorizationTypeBearer = "Bearer"
)

type AuthMiddleware struct {
	JwtManager *auth.JWTManager
}

func NewAuthMiddleware(jwtManager *auth.JWTManager) *AuthMiddleware {
	return &AuthMiddleware{
		JwtManager: jwtManager,
	}
}
