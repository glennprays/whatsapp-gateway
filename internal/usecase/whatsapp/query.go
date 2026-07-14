package whatsapp_usecase

import (
	"context"
	"fmt"
	"sync"
	"time"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

// ttlCache is a tiny concurrency-safe cache with a fixed TTL and lazy expiry. It
// backs the read/query surface: server-hitting reads are cached briefly so
// polling callers hit the cache instead of WhatsApp.
//
// ponytail: lazy expiry only (no background sweep / LRU). Fine for the modest
// key cardinality here (accounts × read-kind × target). Add a sweep if targets
// ever explode.
type ttlCache struct {
	mu  sync.RWMutex
	ttl time.Duration
	m   map[string]cacheEntry
}

type cacheEntry struct {
	val       any
	expiresAt time.Time
}

func newTTLCache(ttl time.Duration) *ttlCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &ttlCache{ttl: ttl, m: make(map[string]cacheEntry)}
}

func (c *ttlCache) get(key string) (any, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		return nil, false
	}
	return e.val, true
}

func (c *ttlCache) set(key string, val any) {
	c.mu.Lock()
	c.m[key] = cacheEntry{val: val, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
}

// queryWithBudget serves a server-hitting read from cache when possible, and
// otherwise spends one per-account budget token before calling fn. A cache hit
// is free; an exhausted budget returns a 429 (ErrTooManyRequests) so callers
// back off instead of hammering WhatsApp. Results are cached on success only.
func queryWithBudget[T any](
	uc *WhatsappMessageUsecase,
	ctx context.Context,
	phoneNumber, cacheKey string,
	fn func() (T, error),
) (T, error) {
	var zero T
	if v, ok := uc.queryCache.get(cacheKey); ok {
		if typed, ok := v.(T); ok {
			return typed, nil
		}
	}

	if err := uc.spendReadBudget(ctx, phoneNumber); err != nil {
		return zero, err
	}

	v, err := fn()
	if err != nil {
		return zero, err
	}
	uc.queryCache.set(cacheKey, v)
	return v, nil
}

// spendReadBudget charges one per-account read token and returns a 429
// (ErrTooManyRequests) domain error when the budget is exhausted. Used both by
// queryWithBudget (cache-miss path) and by reads that bypass the cache (e.g. an
// avatar freshness check via If-None-Match).
func (uc *WhatsappMessageUsecase) spendReadBudget(ctx context.Context, phoneNumber string) error {
	res, err := uc.queryBudget.Allow(ctx, phoneNumber)
	if err == nil && !res.Allowed {
		return errDomain.NewError(errDomain.ErrTooManyRequests,
			fmt.Errorf("read query budget exceeded; retry after %ds", int(res.RetryAfter.Seconds())+1))
	}
	return nil
}

// newQueryBudget builds the per-account read budget limiter from config.
func newQueryBudget(limit, windowSeconds int64) ratelimiter.Limiter {
	if limit <= 0 {
		limit = 30
	}
	window := time.Duration(windowSeconds) * time.Second
	if window <= 0 {
		window = time.Minute
	}
	return ratelimiter.NewMemoryLimiter(ratelimiter.Config{
		Limit:  limit,
		Window: window,
		Prefix: "whatsapp:gateway:readquery:",
	})
}
