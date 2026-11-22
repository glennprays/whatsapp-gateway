package whatsapp

import (
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
)

// IncomingMediaDownloadTimeout derives the per-download incoming-media
// deadline from config. Values <= 0 keep the historical 5-second default.
func IncomingMediaDownloadTimeout(cfg *config.Config) time.Duration {
	if cfg == nil || cfg.IncomingMediaDownloadTimeoutSeconds <= 0 {
		return 5 * time.Second
	}
	return time.Duration(cfg.IncomingMediaDownloadTimeoutSeconds) * time.Second
}
