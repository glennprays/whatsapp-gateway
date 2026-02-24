package handler

import (
	auth_handler "github.com/glennprays/whatsapp-gateway/internal/handler/auth"
	storage_handler "github.com/glennprays/whatsapp-gateway/internal/handler/storage"
	whatsapp_handler "github.com/glennprays/whatsapp-gateway/internal/handler/whatsapp"
)

// ProvideMainHandler initializes main handler
func ProvideMainHandler(
	authHandler *auth_handler.AuthHandler,
	whatsappAuthHandler *whatsapp_handler.WhatsappAuthHandler,
	whatsappWebhookHandler *whatsapp_handler.WhatsappWebhookHandler,
	whatsappMessageHandler *whatsapp_handler.WhatsappMessageHandler,
	storageHandler *storage_handler.StorageHandler,
) *Handler {
	return NewHandler(authHandler, whatsappAuthHandler, whatsappWebhookHandler, whatsappMessageHandler, storageHandler)
}
