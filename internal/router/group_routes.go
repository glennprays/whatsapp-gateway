package router

import (
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func initGroupRoutes(r fiber.Router, h *handler.Handler, authMw *middleware.AuthMiddleware, cfg *config.Config) {
	groupGroup := r.Group("/group")
	groupGroup.Use(authMw.JWTAuthentication())
	// Reads stay registered unconditionally.
	groupGroup.Get("/", h.WhatsappMessageHandler.ListGroups)
	groupGroup.Get("/info", h.WhatsappMessageHandler.GetGroupInfo)

	communityGroup := r.Group("/community")
	communityGroup.Use(authMw.JWTAuthentication())
	communityGroup.Get("/subgroups", h.WhatsappMessageHandler.ListSubGroups)
	communityGroup.Get("/participants", h.WhatsappMessageHandler.ListCommunityParticipants)

	// Mutations, invite links, and join-requests are gated by the master toggle:
	// when off they are never registered, so the whole mutation surface is a 404
	// while the reads above stay up.
	if !cfg.GroupManagementEnabled {
		return
	}
	groupGroup.Post("/", h.WhatsappMessageHandler.CreateGroup)
	groupGroup.Post("/leave", h.WhatsappMessageHandler.LeaveGroup)
	groupGroup.Post("/participants", h.WhatsappMessageHandler.UpdateGroupParticipants)
	groupGroup.Patch("/settings", h.WhatsappMessageHandler.SetGroupSettings)
	groupGroup.Patch("/name", h.WhatsappMessageHandler.SetGroupName)
	groupGroup.Patch("/topic", h.WhatsappMessageHandler.SetGroupTopic)
	groupGroup.Put("/photo", h.WhatsappMessageHandler.SetGroupPhoto)
	groupGroup.Delete("/photo", h.WhatsappMessageHandler.RemoveGroupPhoto)
	groupGroup.Get("/invite", h.WhatsappMessageHandler.GetGroupInviteLink)
	groupGroup.Post("/invite/reset", h.WhatsappMessageHandler.ResetGroupInviteLink)
	groupGroup.Get("/invite/info", h.WhatsappMessageHandler.GetGroupInviteInfo)
	groupGroup.Post("/join", h.WhatsappMessageHandler.JoinGroup)
	groupGroup.Get("/requests", h.WhatsappMessageHandler.ListJoinRequests)
	groupGroup.Post("/requests", h.WhatsappMessageHandler.UpdateJoinRequests)
}
