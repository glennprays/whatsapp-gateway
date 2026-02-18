package database

import (
	"fmt"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

func ProvideRedis(cfg *config.Config, logger *log.Logger) (*redis.Client, error) {
	if cfg.RedisEnabled {
		return NewRedisConnection(logger, cfg.RedisURI)
	}
	return nil, nil
}

func NewRedisConnection(logger *log.Logger, uri string) (*redis.Client, error) {
	redisTraceID := fmt.Sprintf("REDIS-INIT:%s", uuid.New().String())
	opt, err := redis.ParseURL(uri)
	if err != nil {
		logger.Warn(redisTraceID, "Error parsing Redis URL", log.Error(err))
		return nil, err
	}

	rdb := redis.NewClient(opt)

	err = rdb.Ping(rdb.Context()).Err()
	if err != nil {
		logger.Warn(redisTraceID, "Error Ping Redis", log.Error(err))
		return nil, err
	}
	logger.Info(redisTraceID, "Redis connection established successfully", nil)
	return rdb, nil
}
