package infrastructure

import (
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

// ProvideOutboundPacer builds the single shared outbound pacer. Its ban gate
// reads through the manager's persisted session-status store; the disabled-mode
// fallback reuses the legacy per-account limiter so OUTBOUND_PACE_ENABLED=false
// reproduces today's reject behavior. The banUntil adapter is built here because
// pkg/ratelimiter must not import internal/whatsapp.
func ProvideOutboundPacer(cfg *config.Config, mgr whatsapp.Manager, fallback ratelimiter.Limiter) *ratelimiter.Pacer {
	pcfg := ratelimiter.PacerConfig{
		Enabled:         cfg.OutboundPaceEnabled,
		Mode:            cfg.OutboundPaceMode,
		AccountRate:     cfg.OutboundPaceRatePerSecond,
		AccountBurst:    cfg.OutboundPaceBurst,
		MaxWait:         time.Duration(cfg.OutboundPaceMaxWaitSeconds) * time.Second,
		Jitter:          time.Duration(cfg.OutboundPaceJitterMs) * time.Millisecond,
		RecipientLimit:  cfg.OutboundPacePerRecipientRequests,
		RecipientWindow: time.Duration(cfg.OutboundPacePerRecipientWindowSeconds) * time.Second,
		Fallback:        fallback,
	}
	return ratelimiter.NewPacer(pcfg, mgr.BanState)
}
