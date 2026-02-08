package handler

import (
	auth_handler "github.com/glennprays/whatsapp-gateway/internal/handler/auth"
	whatsapp_handler "github.com/glennprays/whatsapp-gateway/internal/handler/whatsapp"
)

type Handler struct {
	AuthHandler             *auth_handler.AuthHandler
	WhatsappAuthHandler     *whatsapp_handler.WhatsappAuthHandler
	WhatsappWebhookHandler  *whatsapp_handler.WhatsappWebhookHandler
	WhatsappMessageHandler  *whatsapp_handler.WhatsappMessageHandler
}

func NewHandler(
	authHandler *auth_handler.AuthHandler,
	whatsappAuthHandler *whatsapp_handler.WhatsappAuthHandler,
	whatsappWebhookHandler *whatsapp_handler.WhatsappWebhookHandler,
	whatsappMessageHandler *whatsapp_handler.WhatsappMessageHandler,
) *Handler {
	return &Handler{
		AuthHandler:             authHandler,
		WhatsappAuthHandler:     whatsappAuthHandler,
		WhatsappWebhookHandler:  whatsappWebhookHandler,
		WhatsappMessageHandler:  whatsappMessageHandler,
	}
}
