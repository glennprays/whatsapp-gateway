package ratelimiter

import (
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/go-redis/redis/v8"
)

func ProvideRateLimiter(cfg *config.Config, rdb *redis.Client) (Limiter, error) {
	limiterCfg := Config{
		Limit:  cfg.RateLimitRequests,
		Window: cfg.GetRateLimitDuration(),
		Prefix: "whatsapp:gateway:ratelimiter:",
	}

	switch cfg.RateLimitProvider {
	case ProviderRedis:
		return NewRedisLimiter(rdb, limiterCfg), nil
	case ProviderMemory:
		return NewMemoryLimiter(limiterCfg), nil
	}

	return NewNoop(), nil
}
