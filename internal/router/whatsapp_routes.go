package router

import (
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initWhatsappRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware) {
	loginGroup := r.Group("/login")
	loginGroup.Use(authMw.JWTAuthentication())
	{
		loginGroup.Post("/qr_code/:format", h.WhatsappAuthHandler.LoginQRCode)
		loginGroup.Post("/pair_code", h.WhatsappAuthHandler.LoginPairCode)
		loginGroup.Get("/status", h.WhatsappAuthHandler.GetLoginStatus)
	}
	r.Post("/logout", authMw.JWTAuthentication(), h.WhatsappAuthHandler.Logout)
	r.Post("/session/reconnect", authMw.JWTAuthentication(), h.WhatsappAuthHandler.Reconnect)
}
