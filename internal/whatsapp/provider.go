package whatsapp

import (
	"database/sql"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
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
) Manager {
	return NewManager(cfg, cfg.WhatsappDatastoreType, db, cipher, logger, queue, mediaDownloader)
}

// ProvideWhatsAppRepository creates repository
func ProvideWhatsAppRepository(db *sql.DB) WhatsAppRepository {
	return NewWhatsappRepository(db)
}

// ProvideWebhookSender creates webhook sender
func ProvideWebhookSender(cipher *cipherx.Cipher) *WebhookSender {
	return NewWebhookSender(cipher)
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
