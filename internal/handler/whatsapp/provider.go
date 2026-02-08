package whatsapp_handler

import (
	customLog "github.com/glennprays/log"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
)

// ProvideWhatsappAuthHandler initializes WhatsApp authentication handler
func ProvideWhatsappAuthHandler(whatsappAuthUsecase *whatsapp_usecase.WhatsappAuthUsecase, logger *customLog.Logger) *WhatsappAuthHandler {
	return NewWhatsappAuthHandler(whatsappAuthUsecase, logger)
}

// ProvideWhatsappWebhookHandler initializes WhatsApp webhook handler
func ProvideWhatsappWebhookHandler(whatsappWebhookUsecase *whatsapp_usecase.WhatsappWebhookUsecase, logger *customLog.Logger) *WhatsappWebhookHandler {
	return NewWhatsappWebhookHandler(whatsappWebhookUsecase, logger)
}

// ProvideWhatsappMessageHandler initializes WhatsApp message handler
func ProvideWhatsappMessageHandler(whatsappMessageUsecase *whatsapp_usecase.WhatsappMessageUsecase, logger *customLog.Logger) *WhatsappMessageHandler {
	return NewWhatsappMessageHandler(whatsappMessageUsecase, logger)
}
