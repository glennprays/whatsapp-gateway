package ratelimiter

import (
	"context"
	"math"
	"sync"
	"time"
)

// TokenBucketConfig configures a per-key token bucket. Rate is the steady
// refill in tokens/second; Burst is the bucket capacity — the largest short
// burst allowed before pacing kicks in.
type TokenBucketConfig struct {
	Rate   float64
	Burst  float64
	Prefix string
}

type tbEntry struct {
	tokens float64
	last   time.Time
}

// TokenBucketLimiter is an in-memory, per-instance token-bucket limiter
// implementing Limiter, one bucket per key (e.g. an account). It is the pacing
// backend.
//
// ponytail: in-memory / per-instance (matches the ReadQueryBudget precedent in
// query.go). A horizontally-scaled fleet paces per node, so the aggregate cap
// is approximate — a Redis token bucket is the upgrade path if a global cap is
// ever proven necessary.
type TokenBucketLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tbEntry
	rate    float64
	burst   float64
	prefix  string
	now     func() time.Time // injectable clock (test seam); defaults to time.Now
}

func NewTokenBucketLimiter(cfg TokenBucketConfig) *TokenBucketLimiter {
	rate := cfg.Rate
	if rate <= 0 {
		rate = 1
	}
	burst := cfg.Burst
	if burst < 1 {
		burst = 1
	}
	return &TokenBucketLimiter{
		buckets: make(map[string]*tbEntry),
		rate:    rate,
		burst:   burst,
		prefix:  cfg.Prefix,
		now:     time.Now,
	}
}

func (t *TokenBucketLimiter) Allow(ctx context.Context, key string) (Result, error) {
	return t.AllowN(ctx, key, 1)
}

// AllowN refills the bucket by the elapsed time (capped at burst) and deducts n
// tokens if available. When short it does NOT deduct and reports how long until
// n tokens would be available in RetryAfter, so a pacer can sleep exactly that
// long. No deduction on denial means a retry loop can never double-spend.
func (t *TokenBucketLimiter) AllowN(ctx context.Context, key string, n int64) (Result, error) {
	if n <= 0 {
		n = 1
	}
	need := float64(n)

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	fullKey := t.prefix + key
	e, ok := t.buckets[fullKey]
	if !ok {
		e = &tbEntry{tokens: t.burst, last: now}
		t.buckets[fullKey] = e
	} else if elapsed := now.Sub(e.last).Seconds(); elapsed > 0 {
		e.tokens = math.Min(t.burst, e.tokens+elapsed*t.rate)
		e.last = now
	}

	if e.tokens >= need {
		e.tokens -= need
		return Result{Allowed: true, Remaining: int64(e.tokens), Limit: int64(t.burst)}, nil
	}

	deficit := need - e.tokens
	retryAfter := time.Duration(deficit / t.rate * float64(time.Second))
	return Result{
		Allowed:    false,
		Remaining:  int64(e.tokens),
		Limit:      int64(t.burst),
		RetryAfter: retryAfter,
		ResetAfter: retryAfter,
	}, nil
}

func (t *TokenBucketLimiter) Reset(ctx context.Context, key string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.buckets, t.prefix+key)
	return nil
}
