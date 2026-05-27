package router

import (
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initWebhookRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware) {
	webhookGroup := r.Group("/webhook")
	webhookGroup.Use(authMw.JWTAuthentication())
	{
		webhookGroup.Get("/", h.WhatsappWebhookHandler.GetWebhookURL)
		webhookGroup.Post("/", h.WhatsappWebhookHandler.SetWebhookURL)
		webhookGroup.Delete("/", h.WhatsappWebhookHandler.DeleteWebhookURL)
	}
}
