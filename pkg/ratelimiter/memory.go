package ratelimiter

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	count     int64
	expiresAt time.Time
}

type MemoryLimiter struct {
	mu     sync.Mutex
	store  map[string]*memoryEntry
	limit  int64
	window time.Duration
	prefix string
}

func NewMemoryLimiter(cfg Config) Limiter {
	return &MemoryLimiter{
		store:  make(map[string]*memoryEntry),
		limit:  cfg.Limit,
		window: cfg.Window,
		prefix: cfg.Prefix,
	}
}

func (m *MemoryLimiter) Allow(ctx context.Context, key string) (Result, error) {
	return m.AllowN(ctx, key, 1)
}

func (m *MemoryLimiter) AllowN(ctx context.Context, key string, n int64) (Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	fullKey := m.prefix + key

	entry, exists := m.store[fullKey]

	if !exists || now.After(entry.expiresAt) {
		entry = &memoryEntry{
			count:     0,
			expiresAt: now.Add(m.window),
		}
		m.store[fullKey] = entry
	}

	if entry.count+n > m.limit {
		return Result{
			Allowed:    false,
			Remaining:  m.limit - entry.count,
			Limit:      m.limit,
			RetryAfter: time.Until(entry.expiresAt),
			ResetAfter: time.Until(entry.expiresAt),
		}, nil
	}

	entry.count += n

	return Result{
		Allowed:    true,
		Remaining:  m.limit - entry.count,
		Limit:      m.limit,
		RetryAfter: 0,
		ResetAfter: time.Until(entry.expiresAt),
	}, nil
}

func (m *MemoryLimiter) Reset(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.store, m.prefix+key)
	return nil
}
