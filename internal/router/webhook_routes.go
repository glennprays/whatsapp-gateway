package router

import (
	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
)

func initWebhookRoutes(r *gin.RouterGroup, h *handler.Handler) {
	webhookGroup := r.Group("/webhook")
	webhookGroup.Use(authMiddleware.JWTAuthentication())
	{
		webhookGroup.GET("/", h.WhatsappWebhookHandler.GetWebhookURL)
		webhookGroup.POST("/", h.WhatsappWebhookHandler.SetWebhookURL)
		webhookGroup.DELETE("/", h.WhatsappWebhookHandler.DeleteWebhookURL)
	}
}
