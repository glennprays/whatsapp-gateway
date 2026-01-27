package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
)

func initWhatsappRoutes(r fiber.Router, h *handler.Handler) {
	loginGroup := r.Group("/login")
	loginGroup.Use(authMiddleware.JWTAuthentication())
	{
		loginGroup.Post("/qr_code/:format", h.WhatsappAuthHandler.LoginQRCode)
		loginGroup.Post("/pair_code", h.WhatsappAuthHandler.LoginPairCode)
		loginGroup.Get("/status", h.WhatsappAuthHandler.GetLoginStatus)
	}
	r.Post("/logout", authMiddleware.JWTAuthentication(), h.WhatsappAuthHandler.Logout)
	r.Post("/session/reconnect", authMiddleware.JWTAuthentication(), h.WhatsappAuthHandler.Reconnect)
}
