package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
)

// Trace IDs for worker pool operations (not tied to user requests)
const (
	traceIDWorkerInit     = "WORKER-INIT"
	traceIDWorkerShutdown = "WORKER-SHUTDOWN"
	TraceIDKey            = "x-trace-id"
	RetryCountKey         = "x-retry-count"
)

type retryTask struct {
	queue   string
	headers amqp.Table
	body    []byte
	delay   time.Duration
}

type MessageHandler func(ctx context.Context, body []byte, headers amqp.Table) error

type WorkerGroup struct {
	queueName string
	workers   int
	publishCh *amqp.Channel
	consumeCh *amqp.Channel
	handler   MessageHandler
	logger    *customLog.Logger
	config    *config.Config
	stopChan  chan struct{}
	wg        sync.WaitGroup
	retryCh   chan retryTask
	ctx       context.Context
	cancel    context.CancelFunc
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
	publishCh *amqp.Channel,
	consumeCh *amqp.Channel,
	handler MessageHandler,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	group := &WorkerGroup{
		queueName: queueName,
		workers:   workerCount,
		publishCh: publishCh,
		consumeCh: consumeCh,
		handler:   handler,
		logger:    wp.logger,
		config:    wp.config,
		stopChan:  make(chan struct{}),
		retryCh:   make(chan retryTask, 1000),
		ctx:       ctx,
		cancel:    cancel,
	}
	group.startRetryScheduler()

	// Start consuming from queue
	msgs, err := consumeCh.Consume(
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

	wp.logger.Info(traceIDWorkerInit, fmt.Sprintf("Started %d workers for queue %s", workerCount, queueName), nil)
	return nil
}

func (wg *WorkerGroup) worker(id int, msgs <-chan amqp.Delivery) {
	defer wg.wg.Done()

	workerName := fmt.Sprintf("%s-worker-%d", wg.queueName, id)
	wg.logger.Info(traceIDWorkerInit, fmt.Sprintf("Worker %s started", workerName), nil)

	for {
		select {
		case <-wg.stopChan:
			wg.logger.Info(traceIDWorkerShutdown, fmt.Sprintf("Worker %s stopping", workerName), nil)
			return

		case msg, ok := <-msgs:
			if !ok {
				wg.logger.Info(traceIDWorkerShutdown, fmt.Sprintf("Worker %s: message channel closed", workerName), nil)
				return
			}

			wg.processMessage(workerName, msg)
		}
	}
}

func (wg *WorkerGroup) processMessage(workerName string, msg amqp.Delivery) {
	ctx := context.Background()
	retryCount := getRetryCount(msg.Headers)
	traceID := GetTraceIDWorkerProcess(msg.Headers)

	headers := msg.Headers
	if headers == nil {
		headers = amqp.Table{}
	}
	headers[TraceIDKey] = traceID

	wg.logger.Debug(
		traceID,
		fmt.Sprintf("Worker %s processing message (retry=%d)", workerName, retryCount),
		nil,
	)

	if err := wg.handler(ctx, msg.Body, headers); err != nil {
		wg.logger.Error(
			traceID,
			fmt.Sprintf("Worker %s: handler error", workerName),
			nil,
			customLog.Error(err),
		)

		task := retryTask{
			queue:   wg.queueName,
			headers: headers,
			body:    msg.Body,
		}

		var rlErr *ratelimiter.RateLimitError
		if errors.As(err, &rlErr) {
			task.delay = rlErr.RetryAfter
			wg.logger.Warn(
				traceID,
				fmt.Sprintf(
					"Worker %s: rate limited, retrying after %s",
					workerName,
					rlErr.RetryAfter,
				),
				nil,
			)
		} else {
			if retryCount >= wg.config.QueueMaxRetries {
				_ = msg.Nack(false, false)
				return
			}

			headers[RetryCountKey] = retryCount + 1

			task.delay = retryBackoff(retryCount)
			wg.logger.Info(
				traceID,
				fmt.Sprintf(
					"Worker %s scheduled retry in %s (attempt %d/%d)",
					workerName,
					task.delay,
					retryCount+1,
					wg.config.QueueMaxRetries,
				),
				nil,
			)
		}

		select {
		case wg.retryCh <- task:
			// durability boundary crossed
			_ = msg.Ack(false)
		default:
			// retry buffer full → DLQ
			wg.logger.Error(
				traceID,
				"retry buffer full, sending to DLQ",
				nil,
			)
			_ = msg.Nack(false, false)
		}
		return
	}

	_ = msg.Ack(false)
}

func retryBackoff(retry int) time.Duration {
	switch retry {
	case 0:
		return 1 * time.Second
	case 1:
		return 5 * time.Second
	case 2:
		return 25 * time.Second
	default:
		return 25 * time.Second
	}
}

func getRetryCount(headers amqp.Table) int {
	if headers == nil {
		return 0
	}

	if count, ok := headers[RetryCountKey]; ok {
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

func GetTraceIDWorkerProcess(headers amqp.Table) string {
	if headers == nil {
		return uuid.New().String()
	}
	if traceID, ok := headers[TraceIDKey]; ok {
		if str, ok := traceID.(string); ok {
			return str
		}
	}
	return uuid.New().String()
}

func (wg *WorkerGroup) startRetryScheduler() {
	go func() {
		for {
			select {
			case task := <-wg.retryCh:
				timer := time.NewTimer(task.delay)

				go func(t retryTask) {
					select {
					case <-timer.C:
						traceID := GetTraceIDWorkerProcess(t.headers)
						err := wg.publishCh.Publish(
							"",
							t.queue,
							false,
							false,
							amqp.Publishing{
								Headers:      t.headers,
								Body:         t.body,
								DeliveryMode: amqp.Persistent,
							},
						)
						if err != nil {
							wg.logger.Error(
								traceID,
								"republish failed",
								nil,
								customLog.Error(err),
							)
						}
					case <-wg.ctx.Done():
						timer.Stop()
					}
				}(task)

			case <-wg.ctx.Done():
				return
			}
		}
	}()
}

func (wp *WorkerPool) Shutdown(timeout time.Duration) {
	wp.logger.Info(traceIDWorkerShutdown, "Shutting down worker pools...", nil)

	wp.mu.RLock()
	defer wp.mu.RUnlock()

	for queueName, group := range wp.pools {
		close(group.stopChan)
		close(group.retryCh)

		// Wait for workers to finish with timeout
		done := make(chan struct{})
		go func() {
			group.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			wp.logger.Info(traceIDWorkerShutdown, fmt.Sprintf("Worker group %s shut down gracefully", queueName), nil)
		case <-time.After(timeout):
			wp.logger.Warn(traceIDWorkerShutdown, fmt.Sprintf("Worker group %s shutdown timeout", queueName), nil)
		}
	}

	wp.logger.Info(traceIDWorkerShutdown, "All worker pools shut down", nil)
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
