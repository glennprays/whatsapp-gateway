package ratelimiter

import (
	"context"
	"time"
)

var ProviderType string

const (
	ProviderRedis  = "redis"
	ProviderMemory = "memory"
	ProviderNoop   = "noop"
)

type Limiter interface {
	Allow(ctx context.Context, key string) (Result, error)
	AllowN(ctx context.Context, key string, n int64) (Result, error)
	Reset(ctx context.Context, key string) error
}

type Result struct {
	Allowed    bool
	Remaining  int64
	Limit      int64
	RetryAfter time.Duration
	ResetAfter time.Duration
}

type Config struct {
	Limit  int64
	Window time.Duration
	Prefix string
}
