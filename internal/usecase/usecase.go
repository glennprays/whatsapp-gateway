package usecase

import (
	auth_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/auth"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
)

// Usecase aggregates all usecases
type Usecase struct {
	AuthUsecase            *auth_usecase.AuthUsecase
	WhatsappAuthUsecase    *whatsapp_usecase.WhatsappAuthUsecase
	WhatsappWebhookUsecase *whatsapp_usecase.WhatsappWebhookUsecase
	WhatsappMessageUsecase *whatsapp_usecase.WhatsappMessageUsecase
}

// NewUsecase creates a new usecase aggregator
func NewUsecase(
	authUsecase *auth_usecase.AuthUsecase,
	whatsappAuthUsecase *whatsapp_usecase.WhatsappAuthUsecase,
	whatsappWebhookUsecase *whatsapp_usecase.WhatsappWebhookUsecase,
	whatsappMessageUsecase *whatsapp_usecase.WhatsappMessageUsecase,
) *Usecase {
	return &Usecase{
		AuthUsecase:            authUsecase,
		WhatsappAuthUsecase:    whatsappAuthUsecase,
		WhatsappWebhookUsecase: whatsappWebhookUsecase,
		WhatsappMessageUsecase: whatsappMessageUsecase,
	}
}
