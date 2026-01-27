package auth_handler

import (
	"errors"
	"net/http"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	authDomain "github.com/glennprays/whatsapp-gateway/domain/auth"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	config          *config.Config
	jwtManager      *auth.JWTManager
	whatsappManager whatsapp.Manager
	logger          *log.Logger
}

func NewAuthHandler(
	cfg *config.Config,
	jwtManager *auth.JWTManager,
	whatsappManager whatsapp.Manager,
	logger *log.Logger,
) *AuthHandler {
	return &AuthHandler{
		config:          cfg,
		jwtManager:      jwtManager,
		whatsappManager: whatsappManager,
		logger:          logger,
	}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)

	var req authDomain.RegistrationRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to bind request data", nil, log.Error(err))
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request data"})
	}

	if req.SecretKey != h.config.BasicAuthSecretKey {
		h.logger.Warn(traceID, "Invalid secret key provided", []log.Field{
			log.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
		})
		err := errors.New("invalid secret key")
		return c.Status(http.StatusForbidden).JSON(httperror.FromError(errDomain.NewError(errDomain.ErrForbidden, err)))
	}

	token, err := h.jwtManager.GenerateTokens(req.PhoneNumber)
	if err != nil {
		h.logger.Error(traceID, "Failed to generate token", []log.Field{
			log.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
		}, log.Error(err))
		err := errors.New("failed to generate token")
		return c.Status(http.StatusInternalServerError).JSON(httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err)))
	}

	h.logger.Info(traceID, "User registered successfully", []log.Field{
		log.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
	})

	return c.Status(http.StatusCreated).JSON(authDomain.RegistrationResponse{
		Token: token,
	})
}
