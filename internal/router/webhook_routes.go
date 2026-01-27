package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
)

func initWebhookRoutes(r fiber.Router, h *handler.Handler) {
	webhookGroup := r.Group("/webhook")
	webhookGroup.Use(authMiddleware.JWTAuthentication())
	{
		webhookGroup.Get("/", h.WhatsappWebhookHandler.GetWebhookURL)
		webhookGroup.Post("/", h.WhatsappWebhookHandler.SetWebhookURL)
		webhookGroup.Delete("/", h.WhatsappWebhookHandler.DeleteWebhookURL)
	}
}
