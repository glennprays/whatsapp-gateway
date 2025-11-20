package router

import (
	"github.com/gin-gonic/gin"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
)

func initWhatsappRoutes(r *gin.RouterGroup, h *handler.Handler) {
	loginGroup := r.Group("/login")
	loginGroup.Use(authMiddleware.JWTAuthentication())
	{
		loginGroup.POST("/qr_code/:format", h.WhatsappAuthHandler.LoginQRCode)
		loginGroup.GET("/status", h.WhatsappAuthHandler.GetLoginStatus)
	}
	r.POST("/logout", authMiddleware.JWTAuthentication(), h.WhatsappAuthHandler.Logout)
	r.POST("/session/reconnect", authMiddleware.JWTAuthentication(), h.WhatsappAuthHandler.Reconnect)
}
