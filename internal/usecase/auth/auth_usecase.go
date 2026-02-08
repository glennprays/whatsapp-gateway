package auth_usecase

import (
	"errors"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	authDomain "github.com/glennprays/whatsapp-gateway/domain/auth"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
)

// AuthUsecase handles authentication business logic
type AuthUsecase struct {
	config          *config.Config
	jwtManager      *auth.JWTManager
	whatsappManager whatsapp.Manager
	logger          *log.Logger
}

// NewAuthUsecase creates a new authentication usecase
func NewAuthUsecase(
	cfg *config.Config,
	jwtManager *auth.JWTManager,
	whatsappManager whatsapp.Manager,
	logger *log.Logger,
) *AuthUsecase {
	return &AuthUsecase{
		config:          cfg,
		jwtManager:      jwtManager,
		whatsappManager: whatsappManager,
		logger:          logger,
	}
}

// Register handles user registration and JWT token generation
func (uc *AuthUsecase) Register(traceID string, req authDomain.RegistrationRequest) (*authDomain.RegistrationResponse, error) {
	// Validate secret key
	if req.SecretKey != uc.config.BasicAuthSecretKey {
		uc.logger.Warn(traceID, "Invalid secret key provided", []log.Field{
			log.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
		})
		return nil, errDomain.NewError(errDomain.ErrForbidden, errors.New("invalid secret key"))
	}

	// Generate JWT token
	token, err := uc.jwtManager.GenerateTokens(req.PhoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to generate token", []log.Field{
			log.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
		}, log.Error(err))
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, errors.New("failed to generate token"))
	}

	uc.logger.Info(traceID, "User registered successfully", []log.Field{
		log.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
	})

	return &authDomain.RegistrationResponse{
		Token: token,
	}, nil
}
