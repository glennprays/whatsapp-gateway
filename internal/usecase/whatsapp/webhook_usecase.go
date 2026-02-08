package whatsapp_usecase

import (
	"context"

	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	log "github.com/sirupsen/logrus"
)

// WhatsappWebhookUsecase handles webhook business logic
type WhatsappWebhookUsecase struct {
	whatsappManager whatsapp.Manager
	logger          *customLog.Logger
}

// NewWhatsappWebhookUsecase creates a new webhook usecase
func NewWhatsappWebhookUsecase(manager whatsapp.Manager, logger *customLog.Logger) *WhatsappWebhookUsecase {
	return &WhatsappWebhookUsecase{
		whatsappManager: manager,
		logger:          logger,
	}
}

// GetWebhookURL retrieves the webhook URL for a phone number
func (uc *WhatsappWebhookUsecase) GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error) {
	webhookURL, err := uc.whatsappManager.GetWebhookURL(ctx, phoneNumber)
	if err != nil {
		log.Errorf("Failed to get webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		return nil, err
	}

	// Return empty string pointer if nil
	if webhookURL == nil {
		webhookURL = new(string)
	}

	return webhookURL, nil
}

// SetWebhookURL sets the webhook URL for a phone number
func (uc *WhatsappWebhookUsecase) SetWebhookURL(ctx context.Context, phoneNumber string, webhook *waDomain.Webhook) error {
	err := uc.whatsappManager.SetWebhookURL(ctx, phoneNumber, webhook)
	if err != nil {
		log.Errorf("Failed to set webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		return err
	}

	return nil
}

// DeleteWebhookURL deletes the webhook URL for a phone number
func (uc *WhatsappWebhookUsecase) DeleteWebhookURL(ctx context.Context, phoneNumber string) error {
	err := uc.whatsappManager.DeleteWebhookURL(ctx, phoneNumber)
	if err != nil {
		log.Errorf("Failed to delete webhook URL for Phone Number: %s, error: %v", whatsapp.MaskedPhoneNumber(phoneNumber), err)
		return err
	}

	return nil
}
