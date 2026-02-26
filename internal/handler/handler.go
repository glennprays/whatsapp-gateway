package handler

import (
	auth_handler "github.com/glennprays/whatsapp-gateway/internal/handler/auth"
	storage_handler "github.com/glennprays/whatsapp-gateway/internal/handler/storage"
	whatsapp_handler "github.com/glennprays/whatsapp-gateway/internal/handler/whatsapp"
)

type Handler struct {
	AuthHandler             *auth_handler.AuthHandler
	WhatsappAuthHandler     *whatsapp_handler.WhatsappAuthHandler
	WhatsappWebhookHandler  *whatsapp_handler.WhatsappWebhookHandler
	WhatsappMessageHandler  *whatsapp_handler.WhatsappMessageHandler
	StorageHandler          *storage_handler.StorageHandler
}

func NewHandler(
	authHandler *auth_handler.AuthHandler,
	whatsappAuthHandler *whatsapp_handler.WhatsappAuthHandler,
	whatsappWebhookHandler *whatsapp_handler.WhatsappWebhookHandler,
	whatsappMessageHandler *whatsapp_handler.WhatsappMessageHandler,
	storageHandler *storage_handler.StorageHandler,
) *Handler {
	return &Handler{
		AuthHandler:             authHandler,
		WhatsappAuthHandler:     whatsappAuthHandler,
		WhatsappWebhookHandler:  whatsappWebhookHandler,
		WhatsappMessageHandler:  whatsappMessageHandler,
		StorageHandler:          storageHandler,
	}
}
