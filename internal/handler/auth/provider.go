package auth_handler

import (
	"github.com/glennprays/log"
	auth_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/auth"
)

// ProvideAuthHandler initializes authentication handler
func ProvideAuthHandler(authUsecase *auth_usecase.AuthUsecase, logger *log.Logger) *AuthHandler {
	return NewAuthHandler(authUsecase, logger)
}
