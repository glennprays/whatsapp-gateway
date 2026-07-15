package router

import (
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initChatRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware) {
	chatGroup := r.Group("/chat")
	chatGroup.Use(authMw.JWTAuthentication())
	chatGroup.Post("/presence", h.WhatsappMessageHandler.SendChatPresence)
}
