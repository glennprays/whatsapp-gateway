package router

import (
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

// initCommunityRoutes registers the community mutation endpoints (link/unlink a
// subgroup). Gated by the same master toggle as group mutations: when off,
// nothing is registered (404). Community reads live in initGroupRoutes and are
// always available.
func initCommunityRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware, cfg *config.Config) {
	if !cfg.GroupManagementEnabled {
		return
	}
	communityGroup := r.Group("/community")
	communityGroup.Use(authMw.JWTAuthentication())
	communityGroup.Post("/subgroups", h.WhatsappMessageHandler.LinkSubGroup)
	communityGroup.Delete("/subgroups", h.WhatsappMessageHandler.UnlinkSubGroup)
}
