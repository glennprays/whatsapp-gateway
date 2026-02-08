package queue

import (
	"context"
	"fmt"
	"sync"
	"time"

	customLog "github.com/glennprays/log"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/glennprays/whatsapp-gateway/config"
)

type MessageHandler func(ctx context.Context, body []byte, headers amqp.Table) error

type WorkerGroup struct {
	queueName string
	workers   int
	channel   *amqp.Channel
	handler   MessageHandler
	logger    *customLog.Logger
	config    *config.Config
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

type WorkerPool struct {
	config *config.Config
	logger *customLog.Logger
	pools  map[string]*WorkerGroup
	mu     sync.RWMutex
}

func (wp *WorkerPool) StartWorkerGroup(
	queueName string,
	workerCount int,
	channel *amqp.Channel,
	handler MessageHandler,
) error {
	group := &WorkerGroup{
		queueName: queueName,
		workers:   workerCount,
		channel:   channel,
		handler:   handler,
		logger:    wp.logger,
		config:    wp.config,
		stopChan:  make(chan struct{}),
	}

	// Start consuming from queue
	msgs, err := channel.Consume(
		queueName,
		"",    // consumer tag (auto-generated)
		false, // auto-ack (manual ack for reliability)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming from %s: %w", queueName, err)
	}

	// Spawn workers
	for i := 0; i < workerCount; i++ {
		group.wg.Add(1)
		go group.worker(i, msgs)
	}

	wp.mu.Lock()
	wp.pools[queueName] = group
	wp.mu.Unlock()

	wp.logger.Info("", fmt.Sprintf("Started %d workers for queue %s", workerCount, queueName), nil)
	return nil
}

func (wg *WorkerGroup) worker(id int, msgs <-chan amqp.Delivery) {
	defer wg.wg.Done()

	workerName := fmt.Sprintf("%s-worker-%d", wg.queueName, id)
	wg.logger.Info("", fmt.Sprintf("Worker %s started", workerName), nil)

	for {
		select {
		case <-wg.stopChan:
			wg.logger.Info("", fmt.Sprintf("Worker %s stopping", workerName), nil)
			return

		case msg, ok := <-msgs:
			if !ok {
				wg.logger.Info("", fmt.Sprintf("Worker %s: message channel closed", workerName), nil)
				return
			}

			wg.processMessage(workerName, msg)
		}
	}
}

func (wg *WorkerGroup) processMessage(workerName string, msg amqp.Delivery) {
	ctx := context.Background()
	retryCount := getRetryCount(msg.Headers)

	wg.logger.Debug("", fmt.Sprintf("Worker %s processing message (retry: %d)", workerName, retryCount), nil)

	// Process message with handler
	err := wg.handler(ctx, msg.Body, msg.Headers)

	if err != nil {
		wg.logger.Error("", fmt.Sprintf("Worker %s: handler error", workerName), nil, customLog.Error(err))

		// Retry with exponential backoff
		if retryCount < wg.config.QueueMaxRetries {
			// Calculate backoff delay: 1s, 5s, 25s
			var backoffDelay time.Duration
			switch retryCount {
			case 0:
				backoffDelay = 1 * time.Second
			case 1:
				backoffDelay = 5 * time.Second
			case 2:
				backoffDelay = 25 * time.Second
			default:
				backoffDelay = 25 * time.Second
			}

			wg.logger.Info("", fmt.Sprintf("Worker %s: retrying in %s (attempt %d/%d)", workerName, backoffDelay, retryCount+1, wg.config.QueueMaxRetries), nil)

			// Sleep for backoff delay
			time.Sleep(backoffDelay)

			// Increment retry count in headers
			if msg.Headers == nil {
				msg.Headers = amqp.Table{}
			}
			msg.Headers["x-retry-count"] = retryCount + 1

			// Requeue message (NACK with requeue)
			if err := msg.Nack(false, true); err != nil {
				wg.logger.Error("", fmt.Sprintf("Worker %s: failed to NACK message", workerName), nil, customLog.Error(err))
			}
			return
		}

		// Max retries exceeded, send to DLQ
		wg.logger.Error("", fmt.Sprintf("Worker %s: max retries exceeded, sending to DLQ", workerName), nil, customLog.Error(err))
		if err := msg.Nack(false, false); err != nil {
			wg.logger.Error("", fmt.Sprintf("Worker %s: failed to NACK message to DLQ", workerName), nil, customLog.Error(err))
		}
		return
	}

	// Success: ACK message
	if err := msg.Ack(false); err != nil {
		wg.logger.Error("", fmt.Sprintf("Worker %s: failed to ACK message", workerName), nil, customLog.Error(err))
	}
}

func getRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}

	if count, ok := headers["x-retry-count"]; ok {
		switch v := count.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		}
	}
	return 0
}

func (wp *WorkerPool) Shutdown(timeout time.Duration) {
	wp.logger.Info("", "Shutting down worker pools...", nil)

	wp.mu.RLock()
	defer wp.mu.RUnlock()

	for queueName, group := range wp.pools {
		close(group.stopChan)

		// Wait for workers to finish with timeout
		done := make(chan struct{})
		go func() {
			group.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			wp.logger.Info("", fmt.Sprintf("Worker group %s shut down gracefully", queueName), nil)
		case <-time.After(timeout):
			wp.logger.Warn("", fmt.Sprintf("Worker group %s shutdown timeout", queueName), nil)
		}
	}

	wp.logger.Info("", "All worker pools shut down", nil)
}

// WorkerManager wraps the queue for graceful shutdown
type WorkerManager struct {
	Queue *RabbitMQQueue
}

func (wm *WorkerManager) Shutdown(timeout time.Duration) error {
	if wm.Queue != nil {
		return wm.Queue.Shutdown(timeout)
	}
	return nil
}
