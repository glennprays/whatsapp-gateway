package ratelimiter

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisLimiter struct {
	client *redis.Client
	limit  int64
	window time.Duration
	prefix string
}

func NewRedisLimiter(client *redis.Client, cfg Config) Limiter {
	return &RedisLimiter{
		client: client,
		limit:  cfg.Limit,
		window: cfg.Window,
		prefix: cfg.Prefix,
	}
}

var luaScript = redis.NewScript(`
local current = redis.call("INCRBY", KEYS[1], ARGV[1])

if tonumber(current) == tonumber(ARGV[1]) then
	redis.call("PEXPIRE", KEYS[1], ARGV[2])
end

local ttl = redis.call("PTTL", KEYS[1])

return {current, ttl}
`)

func (r *RedisLimiter) Allow(ctx context.Context, key string) (Result, error) {
	return r.AllowN(ctx, key, 1)
}

func (r *RedisLimiter) AllowN(ctx context.Context, key string, n int64) (Result, error) {
	fullKey := r.prefix + key
	windowMs := r.window.Milliseconds()

	res, err := luaScript.Run(
		ctx,
		r.client,
		[]string{fullKey},
		n,
		windowMs,
	).Result()
	if err != nil {
		return Result{}, err
	}

	values := res.([]interface{})
	current := values[0].(int64)
	ttlMs := values[1].(int64)

	remaining := r.limit - current
	if remaining < 0 {
		remaining = 0
	}

	allowed := current <= r.limit

	return Result{
		Allowed:    allowed,
		Remaining:  remaining,
		Limit:      r.limit,
		RetryAfter: time.Duration(ttlMs) * time.Millisecond,
		ResetAfter: time.Duration(ttlMs) * time.Millisecond,
	}, nil
}

func (r *RedisLimiter) Reset(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.prefix+key).Err()
}
