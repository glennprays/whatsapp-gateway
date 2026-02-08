package auth_handler

import (
	"net/http"

	"github.com/glennprays/log"
	authDomain "github.com/glennprays/whatsapp-gateway/domain/auth"
	"github.com/glennprays/whatsapp-gateway/internal/httperror"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	auth_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/auth"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	authUsecase *auth_usecase.AuthUsecase
	logger      *log.Logger
}

func NewAuthHandler(
	authUsecase *auth_usecase.AuthUsecase,
	logger *log.Logger,
) *AuthHandler {
	return &AuthHandler{
		authUsecase: authUsecase,
		logger:      logger,
	}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)

	var req authDomain.RegistrationRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error(traceID, "Failed to bind request data", nil, log.Error(err))
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request data"})
	}

	// Call usecase
	response, err := h.authUsecase.Register(traceID, req)
	if err != nil {
		httpErr := httperror.FromError(err)
		return c.Status(httpErr.Status).JSON(httpErr)
	}

	return c.Status(http.StatusCreated).JSON(response)
}
