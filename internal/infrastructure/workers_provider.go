package infrastructure

import (
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/queue"
	queueHandlers "github.com/glennprays/whatsapp-gateway/internal/queue/handlers"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	pkgQueue "github.com/glennprays/whatsapp-gateway/pkg/queue"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

// ProvideQueueWorkers starts worker pools
func ProvideQueueWorkers(
	cfg *config.Config,
	mq domainQueue.MessageQueue,
	repo whatsapp.WhatsAppRepository,
	sender *whatsapp.WebhookSender,
	manager whatsapp.Manager,
	logger *log.Logger,
	jobRepo *queue.JobRepository,
	limiter ratelimiter.Limiter,
	mediaDownloader whatsapp.MediaDownloader,
) (*pkgQueue.WorkerManager, error) {
	if !cfg.RabbitMQEnabled {
		return nil, nil // No workers in direct mode
	}

	rabbitMQ, ok := mq.(*pkgQueue.RabbitMQQueue)
	if !ok {
		return nil, nil
	}

	// Shared duplicate-detection cache (nil when disabled; handlers are
	// nil-safe). Scoped per queue so the same message ID can flow through
	// multiple queues.
	var dedup *pkgQueue.DedupCache
	if cfg.QueueDedupEnabled {
		dedup = pkgQueue.NewDedupCache(time.Duration(cfg.QueueDedupTTLSeconds) * time.Second)
	}

	// Create handlers
	incomingHandler := &queueHandlers.IncomingEventHandler{
		Repository:     repo,
		Publisher:      rabbitMQ,
		Logger:         logger,
		MediaDownloader: mediaDownloader,
		ClientStore:     manager.GetClientStore(),
		Dedup:           dedup,
	}

	webhookHandler := &queueHandlers.WebhookDeliveryHandler{
		Sender: sender,
		Logger: logger,
		Dedup:  dedup,
	}

	outgoingHandler := &queueHandlers.OutgoingMessageHandler{
		Manager:    manager,
		Logger:     logger,
		JobRepo:    jobRepo,
		Repository: repo,
		Sender:     sender,
		Config:     cfg,
		Limiter:    limiter,
		Dedup:      dedup,
	}

	dlqHandler := &queueHandlers.DLQHandler{
		Manager:    manager,
		Repository: repo,
		Sender:     sender,
		Logger:     logger,
		Config:     cfg,
	}

	// Start workers
	if err := rabbitMQ.StartWorkers(
		incomingHandler.Handle,
		webhookHandler.Handle,
		outgoingHandler.Handle,
		dlqHandler.Handle,
	); err != nil {
		return nil, err
	}

	return &pkgQueue.WorkerManager{Queue: rabbitMQ}, nil
}
