package whatsapp_usecase

import (
	"context"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
)

const (
	defaultIncomingLimit = 10
	maxIncomingLimit     = 50
)

// GetIncomingMessages returns the most recent incoming messages for the
// authenticated phone. The limit is clamped to [1, maxIncomingLimit]; a
// non-positive limit falls back to defaultIncomingLimit.
func (uc *WhatsappMessageUsecase) GetIncomingMessages(
	ctx context.Context,
	traceID, phoneNumber string,
	limit int,
) ([]*whatsapp.IncomingMessage, error) {
	if limit <= 0 {
		limit = defaultIncomingLimit
	}
	if limit > maxIncomingLimit {
		limit = maxIncomingLimit
	}
	uc.logger.Info(traceID, "Get incoming messages", []customLog.Field{customLog.Int("limit", limit)})
	return uc.whatsappManager.GetIncomingMessages(ctx, traceID, phoneNumber, limit)
}
