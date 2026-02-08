package queue

import (
	"fmt"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/google/uuid"
)

// ProvideMessageQueue initializes queue (RabbitMQ or fallback)
func ProvideMessageQueue(cfg *config.Config, logger *log.Logger) (domainQueue.MessageQueue, error) {
	traceID := fmt.Sprintf("QUEUE-INIT:%s", uuid.New().String())

	if !cfg.RabbitMQEnabled {
		logger.Info(traceID, "RabbitMQ disabled, using direct processing", nil)
		return NewDirectQueue(logger), nil
	}

	logger.Info(traceID, "RabbitMQ enabled, connecting...", nil)
	mq, err := NewRabbitMQQueue(cfg, logger)
	if err != nil {
		logger.Error(traceID, "Failed to connect to RabbitMQ, falling back to direct processing", nil, log.Error(err))
		return NewDirectQueue(logger), nil // Graceful fallback
	}

	logger.Info(traceID, "RabbitMQ connected successfully", nil)
	return mq, nil
}
