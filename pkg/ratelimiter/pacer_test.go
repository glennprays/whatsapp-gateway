package ratelimiter

import (
	"context"
	"testing"
	"time"
)

// newTestPacer wires a pacer to a shared fake clock; the fake sleep advances
// that clock (and records the waited durations) so paced loops make progress
// without real sleeping.
func newTestPacer(t *testing.T, cfg PacerConfig, banUntil func(string) (time.Time, bool)) (*Pacer, *fakeClock, *[]time.Duration) {
	t.Helper()
	clk := &fakeClock{t: time.Unix(2_000_000, 0)}
	p := NewPacer(cfg, banUntil)
	p.now = clk.Now
	// share the clock with the account bucket so refills track advanced time
	p.account.(*TokenBucketLimiter).now = clk.Now
	var waited []time.Duration
	p.sleep = func(ctx context.Context, d time.Duration) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		waited = append(waited, d)
		clk.Advance(d)
		return nil
	}
	return p, clk, &waited
}

func baseCfg() PacerConfig {
	return PacerConfig{
		Enabled: true, Mode: "pace",
		AccountRate: 1, AccountBurst: 1,
		MaxWait: 10 * time.Second,
	}
}

func TestPacer_PaceModeClearsWithinDeadline(t *testing.T) {
	p, _, waited := newTestPacer(t, baseCfg(), nil)
	ctx := context.Background()

	// Spend the single burst token.
	if err := p.Wait(ctx, "p1", "r1", 1); err != nil {
		t.Fatalf("first send should pass: %v", err)
	}
	// Bucket empty: must sleep ~1s then clear.
	if err := p.Wait(ctx, "p1", "r2", 1); err != nil {
		t.Fatalf("paced send should eventually clear, got %v", err)
	}
	if len(*waited) != 1 {
		t.Fatalf("expected exactly one paced sleep, got %d (%v)", len(*waited), *waited)
	}
	if (*waited)[0] < 900*time.Millisecond || (*waited)[0] > 1100*time.Millisecond {
		t.Fatalf("paced sleep should be ~1s, got %v", (*waited)[0])
	}
}

func TestPacer_DeadlineExceededReturns429(t *testing.T) {
	cfg := baseCfg()
	cfg.AccountRate = 0.01 // ~100s per token; MaxWait 10s can't cover it
	cfg.MaxWait = 10 * time.Second
	p, _, _ := newTestPacer(t, cfg, nil)
	ctx := context.Background()

	p.Wait(ctx, "p1", "r1", 1) // spend burst
	err := p.Wait(ctx, "p1", "r2", 1)
	var rl *RateLimitError
	if !asRateLimit(err, &rl) {
		t.Fatalf("expected *RateLimitError past the deadline, got %v", err)
	}
}

func TestPacer_RejectModeImmediate(t *testing.T) {
	cfg := baseCfg()
	cfg.Mode = "reject"
	p, _, waited := newTestPacer(t, cfg, nil)
	ctx := context.Background()

	p.Wait(ctx, "p1", "r1", 1) // spend burst
	err := p.Wait(ctx, "p1", "r2", 1)
	var rl *RateLimitError
	if !asRateLimit(err, &rl) {
		t.Fatalf("reject mode should return *RateLimitError immediately, got %v", err)
	}
	if len(*waited) != 0 {
		t.Fatalf("reject mode must never sleep, slept %v", *waited)
	}
}

func TestPacer_PerRecipientHardCapNotPaced(t *testing.T) {
	cfg := baseCfg()
	cfg.AccountBurst = 100 // account never the bottleneck
	cfg.AccountRate = 100
	cfg.RecipientLimit = 2
	cfg.RecipientWindow = 60 * time.Second
	p, _, waited := newTestPacer(t, cfg, nil)
	ctx := context.Background()

	if err := p.Wait(ctx, "p1", "r1", 1); err != nil {
		t.Fatalf("1st to r1: %v", err)
	}
	if err := p.Wait(ctx, "p1", "r1", 1); err != nil {
		t.Fatalf("2nd to r1: %v", err)
	}
	// 3rd to the same recipient => hard reject, no sleep.
	err := p.Wait(ctx, "p1", "r1", 1)
	var rl *RateLimitError
	if !asRateLimit(err, &rl) {
		t.Fatalf("per-recipient cap should reject, got %v", err)
	}
	if len(*waited) != 0 {
		t.Fatalf("per-recipient cap must not pace, slept %v", *waited)
	}
	// A different recipient is unaffected.
	if err := p.Wait(ctx, "p1", "r2", 1); err != nil {
		t.Fatalf("different recipient should pass: %v", err)
	}
}

func TestPacer_BanGate(t *testing.T) {
	clkBase := time.Unix(2_000_000, 0)
	banned := true
	until := clkBase.Add(30 * time.Second)
	p, _, _ := newTestPacer(t, baseCfg(), func(string) (time.Time, bool) { return until, banned })
	ctx := context.Background()

	err := p.Wait(ctx, "p1", "r1", 1)
	var rl *RateLimitError
	if !asRateLimit(err, &rl) {
		t.Fatalf("banned account should be rejected, got %v", err)
	}
	if rl.RetryAfter < 29*time.Second || rl.RetryAfter > 31*time.Second {
		t.Fatalf("RetryAfter should be ~ban remaining (30s), got %v", rl.RetryAfter)
	}
	// Ban lifted => clears.
	banned = false
	if err := p.Wait(ctx, "p1", "r1", 1); err != nil {
		t.Fatalf("after ban lifts, send should clear: %v", err)
	}
}

func TestPacer_Disabled_FallsBackToLegacyReject(t *testing.T) {
	cfg := baseCfg()
	cfg.Enabled = false
	cfg.Fallback = NewMemoryLimiter(Config{Limit: 1, Window: time.Minute, Prefix: "leg:"})
	p, _, waited := newTestPacer(t, cfg, nil)
	ctx := context.Background()

	if err := p.Wait(ctx, "p1", "r1", 1); err != nil {
		t.Fatalf("first legacy send should pass: %v", err)
	}
	err := p.Wait(ctx, "p1", "r1", 1)
	var rl *RateLimitError
	if !asRateLimit(err, &rl) {
		t.Fatalf("disabled pacer should fall back to legacy reject, got %v", err)
	}
	if len(*waited) != 0 {
		t.Fatalf("disabled pacer must not pace, slept %v", *waited)
	}
}

func TestPacer_DisabledNoFallback_NoLimiting(t *testing.T) {
	cfg := baseCfg()
	cfg.Enabled = false // no Fallback
	p, _, _ := newTestPacer(t, cfg, nil)
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		if err := p.Wait(ctx, "p1", "r1", 1); err != nil {
			t.Fatalf("disabled+no-fallback should never limit, failed at %d: %v", i, err)
		}
	}
}

func TestPacer_ContextCancelReturnsPromptly(t *testing.T) {
	p, _, _ := newTestPacer(t, baseCfg(), nil)
	// Replace sleep with one that reports cancellation (simulates a hung client).
	p.sleep = func(ctx context.Context, d time.Duration) error { return context.Canceled }
	ctx := context.Background()

	p.Wait(ctx, "p1", "r1", 1) // spend burst
	if err := p.Wait(ctx, "p1", "r2", 1); err != context.Canceled {
		t.Fatalf("cancelled sleep should propagate promptly, got %v", err)
	}
}

// asRateLimit is a tiny errors.As helper kept local to avoid importing errors
// into every assertion.
func asRateLimit(err error, target **RateLimitError) bool {
	if err == nil {
		return false
	}
	if rl, ok := err.(*RateLimitError); ok {
		*target = rl
		return true
	}
	return false
}
