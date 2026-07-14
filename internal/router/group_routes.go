package router

import (
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initGroupRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware) {
	groupGroup := r.Group("/group")
	groupGroup.Use(authMw.JWTAuthentication())
	groupGroup.Get("/", h.WhatsappMessageHandler.ListGroups)
	groupGroup.Get("/info", h.WhatsappMessageHandler.GetGroupInfo)
}
