package whatsapp

import (
	"database/sql"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
)

// ProvideWhatsappManager initializes WhatsApp manager
func ProvideWhatsappManager(
	cfg *config.Config,
	db *sql.DB,
	cipher *cipherx.Cipher,
	logger *log.Logger,
	queue domainQueue.MessageQueue,
) Manager {
	return NewManager(cfg, cfg.WhatsappDatastoreType, db, cipher, logger, queue)
}

// ProvideWhatsAppRepository creates repository
func ProvideWhatsAppRepository(db *sql.DB) WhatsAppRepository {
	return NewWhatsappRepository(db)
}

// ProvideWebhookSender creates webhook sender
func ProvideWebhookSender(cipher *cipherx.Cipher) *WebhookSender {
	return NewWebhookSender(cipher)
}
