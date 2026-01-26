package auth_handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	authDomain "github.com/glennprays/whatsapp-gateway/domain/auth"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
)

type AuthHandler struct {
	config          *config.Config
	jwtManager      *auth.JWTManager
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
}

func NewAuthHandler(
	cfg *config.Config,
	jwtManager *auth.JWTManager,
	whatsappManager whatsapp.Manager,
	logger *customLog.Logger,
) *AuthHandler {
	return &AuthHandler{
		config:          cfg,
		jwtManager:      jwtManager,
		whatsappManager: whatsappManager,
		logger:          logger,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	traceID := middleware.GetTraceID(c)

	var req authDomain.RegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error(traceID, "Failed to bind request data", nil, customLog.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	if req.SecretKey != h.config.BasicAuthSecretKey {
		h.logger.Warn(traceID, "Invalid secret key provided", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
		})
		err := errors.New("invalid secret key")
		c.JSON(http.StatusForbidden, httperror.FromError(errDomain.NewError(errDomain.ErrForbidden, err)))
		return
	}

	token, err := h.jwtManager.GenerateTokens(req.PhoneNumber)
	if err != nil {
		h.logger.Error(traceID, "Failed to generate token", []customLog.Field{
			customLog.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
		}, customLog.Error(err))
		err := errors.New("failed to generate token")
		c.JSON(http.StatusInternalServerError, httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err)))
		return
	}

	h.logger.Info(traceID, "User registered successfully", []customLog.Field{
		customLog.String("phone_number", whatsapp.MaskedPhoneNumber(req.PhoneNumber)),
	})

	c.JSON(http.StatusCreated, authDomain.RegistrationResponse{
		Token: token,
	})
}
