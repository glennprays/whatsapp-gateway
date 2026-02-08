package auth

import (
	"github.com/glennprays/whatsapp-gateway/config"
)

// ProvideJWTManager initializes JWT manager
func ProvideJWTManager(cfg *config.Config) *JWTManager {
	return NewJWTManager(cfg.JwtSecret, cfg.JwtIssuer, cfg.GetJwtDuration())
}
