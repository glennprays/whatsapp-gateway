package auth_handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/config"
	authDomain "github.com/glennprays/whatsapp-gateway/domain/auth"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
	log "github.com/sirupsen/logrus"
)

type AuthHandler struct {
	config     *config.Config
	jwtManager *auth.JWTManager
}

func NewAuthHandler(cfg *config.Config, jwtManager *auth.JWTManager) *AuthHandler {
	return &AuthHandler{
		config:     cfg,
		jwtManager: jwtManager,
	}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req authDomain.RegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf("Failed to bind request data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	if req.SecretKey != h.config.SecretKey {
		log.Warnf("Invalid secret key provided for phone number %s", req.PhoneNumber)
		err := errors.New("invalid secret key")
		c.JSON(http.StatusForbidden, httperror.FromError(errDomain.NewError(errDomain.ErrForbidden, err)))
		return
	}

	token, err := h.jwtManager.GenerateTokens(req.PhoneNumber)
	if err != nil {
		log.Errorf("Failed to generate token for phone number %s: %v", req.PhoneNumber, err)
		err := errors.New("failed to generate token")
		c.JSON(http.StatusInternalServerError, httperror.FromError(errDomain.NewError(errDomain.ErrInternalFailure, err)))
		return
	}

	c.JSON(http.StatusCreated, authDomain.RegistrationResponse{
		Token: token,
	})
}
