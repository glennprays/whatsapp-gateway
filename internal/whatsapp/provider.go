package whatsapp

import (
	"database/sql"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	"go.mau.fi/whatsmeow"
)

// ProvideWhatsappManager initializes WhatsApp manager
func ProvideWhatsappManager(
	cfg *config.Config,
	db *sql.DB,
	cipher *cipherx.Cipher,
	logger *log.Logger,
	queue domainQueue.MessageQueue,
	mediaDownloader MediaDownloader,
) (Manager, error) {
	return NewManager(cfg, cfg.WhatsappDatastoreType, db, cipher, logger, queue, mediaDownloader)
}

// ProvideWhatsAppRepository creates repository
func ProvideWhatsAppRepository(db *sql.DB) WhatsAppRepository {
	return NewWhatsappRepository(db)
}

// ProvideWebhookSender creates webhook sender
func ProvideWebhookSender(cipher *cipherx.Cipher, logger *log.Logger, cfg *config.Config) *WebhookSender {
	return NewWebhookSender(cipher, logger, cfg)
}

// ProvideMediaDownloader creates media downloader
func ProvideMediaDownloader(
	storage domainStorage.Storage,
	cfg *config.Config,
	logger *log.Logger,
) MediaDownloader {
	getClientFunc := func(phoneNumber string) *whatsmeow.Client {
		return clients.Get(phoneNumber)
	}
	return NewMediaDownloader(storage, cfg, logger, getClientFunc)
}
