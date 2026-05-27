package router

import (
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initMessageRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware) {
	messageGroup := r.Group("/message")
	messageGroup.Use(authMw.JWTAuthentication())
	{
		messageGroup.Post("/text", h.WhatsappMessageHandler.SendTextMessage)
		messageGroup.Post("/image", h.WhatsappMessageHandler.SendImageMessage)
		messageGroup.Post("/react", h.WhatsappMessageHandler.ReactToMessage)
		messageGroup.Delete("/", h.WhatsappMessageHandler.DeleteMessage)
		messageGroup.Put("/", h.WhatsappMessageHandler.EditMessage)
		messageGroup.Get("/job/:job_id", h.WhatsappMessageHandler.GetJobStatus)
		messageGroup.Get("/incoming", h.WhatsappMessageHandler.GetIncomingMessages)
	}
}
