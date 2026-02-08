package cipherx

import (
	"github.com/glennprays/whatsapp-gateway/config"
)

// ProvideCipher initializes encryption cipher
func ProvideCipher(cfg *config.Config) *Cipher {
	return NewCipher(cfg.WhatsappWebhookHmacEncryptionMasterKey)
}
