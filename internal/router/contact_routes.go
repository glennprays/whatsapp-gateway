package router

import (
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initContactRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware) {
	contactGroup := r.Group("/contact")
	contactGroup.Use(authMw.JWTAuthentication())
	contactGroup.Get("/check", h.WhatsappMessageHandler.CheckNumber)
	contactGroup.Get("/", h.WhatsappMessageHandler.ListContacts)
	contactGroup.Get("/info", h.WhatsappMessageHandler.GetContactInfo)
	// Avatar accepts any chat (user or group JID) via ?chat=.
	contactGroup.Get("/avatar", h.WhatsappMessageHandler.GetAvatar)
}
