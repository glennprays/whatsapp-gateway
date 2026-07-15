package whatsapp_usecase

import (
	"context"

	customLog "github.com/glennprays/log"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
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

// ListWebhookSubscriptions returns all webhook subscriptions for a phone number.
func (uc *WhatsappWebhookUsecase) ListWebhookSubscriptions(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.WebhookSubscription, error) {
	subs, err := uc.whatsappManager.ListWebhookSubscriptions(ctx, traceID, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to list webhook subscriptions for Phone Number: "+whatsapp.MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return nil, err
	}
	return subs, nil
}

// SetWebhookSubscription registers/updates one webhook subscription.
func (uc *WhatsappWebhookUsecase) SetWebhookSubscription(ctx context.Context, traceID string, phoneNumber string, webhook *waDomain.Webhook) error {
	err := uc.whatsappManager.SetWebhookSubscription(ctx, traceID, phoneNumber, webhook)
	if err != nil {
		uc.logger.Error(traceID, "Failed to set webhook subscription for Phone Number: "+whatsapp.MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return err
	}
	return nil
}

// DeleteWebhookSubscription removes a single subscription by URL.
func (uc *WhatsappWebhookUsecase) DeleteWebhookSubscription(ctx context.Context, traceID string, phoneNumber string, url string) error {
	err := uc.whatsappManager.DeleteWebhookSubscription(ctx, traceID, phoneNumber, url)
	if err != nil {
		uc.logger.Error(traceID, "Failed to delete webhook subscription for Phone Number: "+whatsapp.MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return err
	}
	return nil
}

// DeleteAllWebhookSubscriptions removes every subscription for a phone number
// (legacy no-body DELETE semantics).
func (uc *WhatsappWebhookUsecase) DeleteAllWebhookSubscriptions(ctx context.Context, traceID string, phoneNumber string) error {
	err := uc.whatsappManager.DeleteAllWebhookSubscriptions(ctx, traceID, phoneNumber)
	if err != nil {
		uc.logger.Error(traceID, "Failed to delete webhook subscriptions for Phone Number: "+whatsapp.MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return err
	}
	return nil
}
