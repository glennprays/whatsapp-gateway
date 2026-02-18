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

type RateLimitError struct {
	RetryAfter time.Duration
	Limit      int64
	Remaining  int64
}

func (e *RateLimitError) Error() string {
	return "rate limit exceeded"
}

func (e *RateLimitError) BuildError(res Result) error {
	return &RateLimitError{
		RetryAfter: res.RetryAfter,
		Limit:      res.Limit,
		Remaining:  res.Remaining,
	}
}

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
