package ratelimiter

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// PacerConfig configures outbound pacing. See config.go OUTBOUND_PACE_* keys.
type PacerConfig struct {
	Enabled         bool
	Mode            string        // "pace" (block up to MaxWait) | "reject" (immediate)
	AccountRate     float64       // account bucket refill, tokens/sec
	AccountBurst    float64       // account bucket capacity
	MaxWait         time.Duration // pace-mode block deadline
	Jitter          time.Duration // random extra gap per paced sleep (traffic non-periodic)
	RecipientLimit  int64         // hard per-(account,recipient) cap
	RecipientWindow time.Duration // window for the per-recipient cap
	Fallback        Limiter       // used when disabled: legacy per-account reject (nil => no limiting)
}

// Pacer spaces outbound WhatsApp operations to stay under anti-spam thresholds.
//
// Design: per-recipient is a HARD reject (repeat sends to one recipient is the
// #1 ban trigger — stop, don't block-and-eventually-send); per-account is a
// blocking token bucket (aggregate throughput just needs spacing). Only the
// account bucket blocks, so there is no two-bucket over-wait / double-spend.
// A ban gate (read-through of the persisted session-status ban) short-circuits
// everything while an account is temporarily banned.
type Pacer struct {
	enabled      bool
	mode         string
	account      Limiter // token bucket, key = phone
	accountBurst float64 // effective bucket capacity; n>burst is never satisfiable
	recipient    Limiter // fixed-window reject, key = phone+":"+recipient
	fallback     Limiter // disabled-mode legacy reject
	maxWait      time.Duration
	jitter       time.Duration
	banUntil     func(phone string) (time.Time, bool)

	sleep func(ctx context.Context, d time.Duration) error // test seam
	now   func() time.Time                                 // test seam
	rndMu sync.Mutex
	rnd   *rand.Rand
}

// NewPacer builds a Pacer. banUntil reports an account's ban deadline (banned ==
// true with a future time while the account is temporarily banned); it is
// nil-safe. The banUntil adapter is wired in internal/infrastructure so pkg does
// not import internal.
func NewPacer(cfg PacerConfig, banUntil func(string) (time.Time, bool)) *Pacer {
	mode := cfg.Mode
	if mode != "reject" {
		mode = "pace"
	}
	var recipient Limiter
	if cfg.RecipientLimit > 0 && cfg.RecipientWindow > 0 {
		recipient = NewMemoryLimiter(Config{Limit: cfg.RecipientLimit, Window: cfg.RecipientWindow, Prefix: "rcpt:"})
	}
	account := NewTokenBucketLimiter(TokenBucketConfig{Rate: cfg.AccountRate, Burst: cfg.AccountBurst, Prefix: "acct:"})
	return &Pacer{
		enabled:      cfg.Enabled,
		mode:         mode,
		account:      account,
		accountBurst: account.Burst(), // effective (clamped) capacity
		recipient:    recipient,
		fallback:     cfg.Fallback,
		maxWait:      cfg.MaxWait,
		jitter:       cfg.Jitter,
		banUntil:     banUntil,
		sleep:        defaultSleep,
		now:          time.Now,
		rnd:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Wait clears an outbound op for account `phone` to `recipient`, spending n
// account tokens (one per participant for bulk group-add). It returns nil when
// cleared, a *RateLimitError when paced out / capped / banned (map to 429 +
// Retry-After), or ctx.Err() if the request is cancelled mid-block.
func (p *Pacer) Wait(ctx context.Context, phone, recipient string, n int64) error {
	if !p.enabled {
		return p.fallbackReject(ctx, phone, n)
	}

	// 1. Ban gate: hard block while temporarily banned (auto-resumes on expiry,
	// no timer — the gate is re-checked on every send). Restart-safe because the
	// ban deadline is read through from the persisted session-status store.
	if p.banUntil != nil {
		if until, banned := p.banUntil(phone); banned {
			if d := until.Sub(p.now()); d > 0 {
				return &RateLimitError{RetryAfter: d}
			}
		}
	}

	// A request larger than the account burst can never accrue enough tokens —
	// fail fast with a 429 instead of blocking the whole MaxWait then rejecting
	// anyway (and before spending any recipient budget on a doomed op). Callers
	// pace one op at a time (n=1), so this only guards against future misuse.
	if float64(n) > p.accountBurst {
		return &RateLimitError{RetryAfter: 0, Limit: int64(p.accountBurst)}
	}

	// 2. Per-recipient hard cap — reject, never paced. Checked before the account
	// wait so repeat-to-one-recipient (the #1 ban trigger) is rejected fast rather
	// than blocked-then-sent.
	// ponytail: this consumes the recipient slot before the account bucket, so a
	// send that later times out on the account wait burns one slot it never used.
	// The effect is transient (self-heals within RecipientWindow) and bounded by
	// the cap; a peek-then-commit (or refund on account failure) is the upgrade
	// path if that transient over-count is ever proven to matter.
	if p.recipient != nil && recipient != "" {
		res, err := p.recipient.AllowN(ctx, phone+":"+recipient, n)
		if err != nil {
			return err
		}
		if !res.Allowed {
			return &RateLimitError{RetryAfter: res.RetryAfter, Limit: res.Limit, Remaining: res.Remaining}
		}
	}

	// 3. Account token bucket — the single blocking dimension.
	deadline := p.now().Add(p.maxWait)
	for {
		res, err := p.account.AllowN(ctx, phone, n)
		if err != nil {
			return err
		}
		if res.Allowed {
			return nil
		}
		if p.mode == "reject" {
			return &RateLimitError{RetryAfter: res.RetryAfter, Limit: res.Limit, Remaining: res.Remaining}
		}
		remaining := deadline.Sub(p.now())
		if remaining <= 0 {
			return &RateLimitError{RetryAfter: res.RetryAfter}
		}
		wait := min(res.RetryAfter+p.randJitter(), remaining)
		if err := p.sleep(ctx, wait); err != nil {
			return err // context cancelled: return promptly, no leaked goroutine
		}
	}
}

func (p *Pacer) fallbackReject(ctx context.Context, phone string, n int64) error {
	if p.fallback == nil {
		return nil
	}
	res, err := p.fallback.AllowN(ctx, phone, n)
	if err != nil {
		return err
	}
	if !res.Allowed {
		return &RateLimitError{RetryAfter: res.RetryAfter, Limit: res.Limit, Remaining: res.Remaining}
	}
	return nil
}

// Drain empties an account's token bucket (called on ban entry so no burst is
// available the instant a ban lifts). The ban gate already blocks sends, so
// this is belt-and-suspenders.
func (p *Pacer) Drain(phone string) {
	if p.account != nil {
		_ = p.account.Reset(context.Background(), phone)
	}
}

func (p *Pacer) randJitter() time.Duration {
	if p.jitter <= 0 {
		return 0
	}
	p.rndMu.Lock()
	d := time.Duration(p.rnd.Int63n(int64(p.jitter)))
	p.rndMu.Unlock()
	return d
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
