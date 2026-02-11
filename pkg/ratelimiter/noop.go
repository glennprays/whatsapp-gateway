package ratelimiter

import (
	"context"
)

type NoopLimiter struct{}

func NewNoop() Limiter {
	return &NoopLimiter{}
}

func (n *NoopLimiter) Allow(ctx context.Context, key string) (Result, error) {
	return Result{
		Allowed:   true,
		Remaining: -1,
		Limit:     -1,
	}, nil
}

func (n *NoopLimiter) AllowN(ctx context.Context, key string, nTokens int64) (Result, error) {
	return n.Allow(ctx, key)
}

func (n *NoopLimiter) Reset(ctx context.Context, key string) error {
	return nil
}
