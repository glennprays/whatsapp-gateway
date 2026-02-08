package auth_usecase

import (
	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
)

// ProvideAuthUsecase initializes authentication usecase
func ProvideAuthUsecase(
	cfg *config.Config,
	jwtManager *auth.JWTManager,
	whatsappManager whatsapp.Manager,
	logger *log.Logger,
) *AuthUsecase {
	return NewAuthUsecase(cfg, jwtManager, whatsappManager, logger)
}
