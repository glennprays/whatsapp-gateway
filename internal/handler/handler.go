package handler

import auth_handler "github.com/glennprays/whatsapp-gateway/internal/handler/auth"

type Handler struct {
	AuthHandler *auth_handler.AuthHandler
}

func NewHandler(authHandler *auth_handler.AuthHandler) *Handler {
	return &Handler{
		AuthHandler: authHandler,
	}
}
