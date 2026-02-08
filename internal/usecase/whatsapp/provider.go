package whatsapp_usecase

import (
	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/queue"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
)

// ProvideWhatsappAuthUsecase initializes WhatsApp authentication usecase
func ProvideWhatsappAuthUsecase(whatsappManager whatsapp.Manager, logger *customLog.Logger) *WhatsappAuthUsecase {
	return NewWhatsappAuthUsecase(whatsappManager, logger)
}

// ProvideWhatsappWebhookUsecase initializes WhatsApp webhook usecase
func ProvideWhatsappWebhookUsecase(whatsappManager whatsapp.Manager, logger *customLog.Logger) *WhatsappWebhookUsecase {
	return NewWhatsappWebhookUsecase(whatsappManager, logger)
}

// ProvideWhatsappMessageUsecase initializes WhatsApp message usecase
func ProvideWhatsappMessageUsecase(
	whatsappManager whatsapp.Manager,
	logger *customLog.Logger,
	queue domainQueue.MessageQueue,
	jobRepo *queue.JobRepository,
	whatsappRepo whatsapp.WhatsAppRepository,
	webhookSender *whatsapp.WebhookSender,
	cfg *config.Config,
) *WhatsappMessageUsecase {
	return NewWhatsappMessageUsecase(whatsappManager, logger, queue, jobRepo, whatsappRepo, webhookSender, cfg)
}
