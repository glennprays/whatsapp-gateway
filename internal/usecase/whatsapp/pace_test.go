package whatsapp_usecase

import (
	"context"
	"testing"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

// TestPaceNilPacerIsNoop proves a usecase without a pacer never blocks a send.
func TestPaceNilPacerIsNoop(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	for i := 0; i < 50; i++ {
		if err := uc.pace(context.Background(), "p1", "r1", 1); err != nil {
			t.Fatalf("nil pacer must be a no-op, failed at %d: %v", i, err)
		}
	}
}

// TestPaceMapsRateLimitTo429 proves the pace helper maps a paced-out result to
// the ErrTooManyRequests domain error (reject mode, burst 1: the 2nd op trips).
// It also stands in for the interim action cap being gone: react/mark-read/
// typing/edit/delete all route through this same helper now.
func TestPaceMapsRateLimitTo429(t *testing.T) {
	pacer := ratelimiter.NewPacer(ratelimiter.PacerConfig{
		Enabled: true, Mode: "reject", AccountRate: 1, AccountBurst: 1,
	}, nil)
	uc := &WhatsappMessageUsecase{pacer: pacer}
	ctx := context.Background()

	if err := uc.pace(ctx, "p1", "r1", 1); err != nil {
		t.Fatalf("first op should pass: %v", err)
	}
	// Burst exhausted; reject mode returns immediately.
	err := uc.pace(ctx, "p1", "r2", 1)
	assertServiceErr(t, err, errDomain.ErrTooManyRequests)
}
