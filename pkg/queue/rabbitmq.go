package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
)

// Trace IDs for RabbitMQ operations (not tied to user requests)
var (
	traceIDRabbitMQInit   string
	traceIDRabbitMQHealth string
	traceIDRabbitMQClose  string
)

func init() {
	traceIDRabbitMQInit = fmt.Sprintf("RABBITMQ-INIT:%s", uuid.New().String())
	traceIDRabbitMQHealth = fmt.Sprintf("RABBITMQ-HEALTH:%s", uuid.New().String())
	traceIDRabbitMQClose = fmt.Sprintf("RABBITMQ-CLOSE:%s", uuid.New().String())
}

// handlerRegistration remembers a queue's worker count and handler so
// consumers can be restarted with fresh channels after a reconnect.
type handlerRegistration struct {
	workers int
	handler MessageHandler
}

type RabbitMQQueue struct {
	config     *config.Config
	logger     *customLog.Logger
	conn       *amqp.Connection
	publishCh  *amqp.Channel
	consumeCh  *amqp.Channel
	mu         sync.RWMutex
	healthy    bool
	closing    bool
	stopCh     chan struct{}
	workerPool *WorkerPool
	handlers   map[string]handlerRegistration
}

func NewRabbitMQQueue(cfg *config.Config, logger *customLog.Logger) (*RabbitMQQueue, error) {
	mq := &RabbitMQQueue{
		config:  cfg,
		logger:  logger,
		healthy: false,
		stopCh:  make(chan struct{}),
	}

	if err := mq.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	if err := mq.setupTopology(); err != nil {
		return nil, fmt.Errorf("failed to setup RabbitMQ topology: %w", err)
	}

	mq.healthy = true
	logger.Info(traceIDRabbitMQInit, "RabbitMQ connection established", nil)

	// Monitor connection health
	go mq.monitorConnection()

	return mq, nil
}

func (mq *RabbitMQQueue) connect() error {
	// Close any stale previous connection so repeated reconnect attempts
	// never leak sockets.
	mq.mu.RLock()
	oldConn := mq.conn
	mq.mu.RUnlock()
	if oldConn != nil && !oldConn.IsClosed() {
		_ = oldConn.Close()
	}

	conn, err := amqp.DialConfig(mq.config.RabbitMQURL, amqp.Config{
		Properties: amqp.Table{
			"connection_name": mq.config.RabbitMQConnectionName,
		},
	})
	if err != nil {
		return err
	}

	consumeCh, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}

	publishCh, err := conn.Channel()
	if err != nil {
		consumeCh.Close()
		conn.Close()
		return err
	}

	// QoS ONLY on consume channel
	if err := consumeCh.Qos(
		mq.config.RabbitMQPrefetchCount,
		0,
		false,
	); err != nil {
		publishCh.Close()
		consumeCh.Close()
		conn.Close()
		return err
	}

	// Put the publish channel in confirm mode so publishes can be verified
	// against broker acknowledgements. Re-applied automatically on reconnect
	// since connect() runs each time.
	if mq.config.RabbitMQPublishConfirm {
		if err := publishCh.Confirm(false); err != nil {
			publishCh.Close()
			consumeCh.Close()
			conn.Close()
			return fmt.Errorf("failed to enable publisher confirms: %w", err)
		}
	}

	mq.mu.Lock()
	mq.conn = conn
	mq.consumeCh = consumeCh
	mq.publishCh = publishCh
	mq.mu.Unlock()

	return nil
}

func (mq *RabbitMQQueue) setupTopology() error {
	mq.mu.RLock()
	ch := mq.consumeCh
	mq.mu.RUnlock()

	// Declare main exchange
	if err := ch.ExchangeDeclare(
		ExchangeName,
		ExchangeType,
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare dead letter exchange
	if err := ch.ExchangeDeclare(
		ExchangeDLX,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare DLX: %w", err)
	}

	// Define queues with their DLQ
	queues := []struct {
		name       string
		routingKey string
		dlq        string
		retry      string
	}{
		{QueueIncomingEvents, RoutingKeyIncomingEvent, DLQIncomingEvents, RetryIncomingEvents},
		{QueueWebhookDelivery, RoutingKeyWebhook, DLQWebhookDelivery, RetryWebhookDelivery},
		{QueueOutgoingMessages, RoutingKeyOutgoingMsg, DLQOutgoingMessages, RetryOutgoingMessages},
	}

	for _, q := range queues {
		// Declare DLQ
		if _, err := ch.QueueDeclare(
			q.dlq,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			nil,
		); err != nil {
			return fmt.Errorf("failed to declare DLQ %s: %w", q.dlq, err)
		}

		// Bind DLQ to DLX
		if err := ch.QueueBind(
			q.dlq,
			q.dlq,
			ExchangeDLX,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind DLQ %s: %w", q.dlq, err)
		}

		// Declare main queue with DLX configuration
		if _, err := ch.QueueDeclare(
			q.name,
			true,
			false,
			false,
			false,
			amqp.Table{
				"x-dead-letter-exchange":    ExchangeDLX,
				"x-dead-letter-routing-key": q.dlq,
			},
		); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", q.name, err)
		}

		// Declare retry queue
		if _, err := ch.QueueDeclare(
			q.retry,
			true,
			false,
			false,
			false,
			amqp.Table{
				"x-dead-letter-exchange":    ExchangeName,
				"x-dead-letter-routing-key": q.routingKey,
			},
		); err != nil {
			return fmt.Errorf("failed to declare retry queue %s: %w", q.retry, err)
		}

		// Bind retry queue to exchange with retry routing key
		if err := ch.QueueBind(
			q.retry,
			fmt.Sprintf("%s.retry", q.name),
			ExchangeName,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind retry queue %s: %w", q.retry, err)
		}

		// Bind queue to exchange
		if err := ch.QueueBind(
			q.name,
			q.routingKey,
			ExchangeName,
			false,
			nil,
		); err != nil {
			return fmt.Errorf("failed to bind queue %s: %w", q.name, err)
		}
	}

	return nil
}

func (mq *RabbitMQQueue) monitorConnection() {
	for {
		mq.mu.RLock()
		conn := mq.conn
		mq.mu.RUnlock()

		if conn == nil {
			return
		}

		closeChan := conn.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case <-mq.stopCh:
			return
		case err := <-closeChan:
			// A nil error means the connection was closed gracefully (e.g.
			// via Shutdown); do not attempt to reconnect, just stop.
			if err == nil {
				return
			}

			mq.logger.Error(traceIDRabbitMQHealth, "RabbitMQ connection closed", nil, customLog.Error(err))
			mq.mu.Lock()
			mq.healthy = false
			mq.mu.Unlock()

			// Reconnect, then restart consumers on the fresh channels. If
			// restarting consumers fails the connection is likely broken
			// again, so go back to reconnecting.
			for {
				if !mq.reconnectLoop() {
					return
				}
				if err := mq.restartConsumers(); err != nil {
					mq.logger.Error(traceIDRabbitMQHealth, "Failed to restart consumers after reconnect", nil, customLog.Error(err))
					mq.mu.Lock()
					mq.healthy = false
					mq.mu.Unlock()
					continue
				}
				break
			}
		}
	}
}

// reconnectLoop retries until the connection and topology are restored.
// It returns false if shutdown was requested while reconnecting.
func (mq *RabbitMQQueue) reconnectLoop() bool {
	mq.logger.Info(traceIDRabbitMQHealth, "Attempting to reconnect to RabbitMQ...", nil)
	delay := time.Duration(mq.config.RabbitMQReconnectDelaySeconds) * time.Second

	for {
		select {
		case <-mq.stopCh:
			return false
		case <-time.After(delay):
		}

		if err := mq.connect(); err != nil {
			mq.logger.Error(traceIDRabbitMQHealth, "Failed to reconnect to RabbitMQ", nil, customLog.Error(err))
			continue
		}

		if err := mq.setupTopology(); err != nil {
			mq.logger.Error(traceIDRabbitMQHealth, "Failed to setup topology after reconnect", nil, customLog.Error(err))
			continue
		}

		mq.mu.Lock()
		mq.healthy = true
		mq.mu.Unlock()
		mq.logger.Info(traceIDRabbitMQHealth, "Successfully reconnected to RabbitMQ", nil)
		return true
	}
}

func (mq *RabbitMQQueue) PublishIncomingEvent(ctx context.Context, event domainQueue.IncomingEventMessage) error {
	return mq.publish(ctx, RoutingKeyIncomingEvent, event)
}

func (mq *RabbitMQQueue) PublishOutgoingMessage(ctx context.Context, job domainQueue.OutgoingMessageJob) error {
	return mq.publish(ctx, RoutingKeyOutgoingMsg, job)
}

func (mq *RabbitMQQueue) PublishWebhookDelivery(ctx context.Context, msg domainQueue.WebhookDeliveryMessage) error {
	return mq.publish(ctx, RoutingKeyWebhook, msg)
}

func (mq *RabbitMQQueue) publish(ctx context.Context, routingKey string, payload interface{}) error {
	mq.mu.RLock()
	ch := mq.publishCh
	healthy := mq.healthy
	mq.mu.RUnlock()

	if !healthy {
		return fmt.Errorf("RabbitMQ connection is not healthy")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	publishing := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
	}

	if mq.config.RabbitMQPublishConfirm {
		confirmCtx, cancel := context.WithTimeout(ctx, time.Duration(mq.config.RabbitMQConfirmTimeoutSeconds)*time.Second)
		defer cancel()

		conf, err := ch.PublishWithDeferredConfirmWithContext(
			confirmCtx, ExchangeName, routingKey, false, false, publishing,
		)
		if err != nil {
			return fmt.Errorf("failed to publish message: %w", err)
		}

		acked, err := conf.WaitContext(confirmCtx)
		if err != nil {
			return fmt.Errorf("waiting for publish confirm: %w", err)
		}
		if !acked {
			return fmt.Errorf("publish nacked by broker")
		}
		return nil
	}

	publishCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return ch.PublishWithContext(
		publishCtx,
		ExchangeName,
		routingKey,
		false, // mandatory
		false, // immediate
		publishing,
	)
}

func (mq *RabbitMQQueue) IsHealthy() bool {
	mq.mu.RLock()
	defer mq.mu.RUnlock()
	return mq.healthy
}

func (mq *RabbitMQQueue) StartWorkers(
	incomingHandler MessageHandler,
	webhookHandler MessageHandler,
	outgoingHandler MessageHandler,
) error {
	// Remember registrations so consumers can be restarted with fresh
	// channels after a reconnect.
	mq.mu.Lock()
	mq.handlers = map[string]handlerRegistration{
		QueueIncomingEvents:   {workers: mq.config.WorkerIncomingEvents, handler: incomingHandler},
		QueueWebhookDelivery:  {workers: mq.config.WorkerWebhookDelivery, handler: webhookHandler},
		QueueOutgoingMessages: {workers: mq.config.WorkerOutgoingMessages, handler: outgoingHandler},
	}
	mq.mu.Unlock()

	return mq.startConsumers()
}

// startConsumers builds a fresh worker pool consuming on the current
// channels for every registered handler.
func (mq *RabbitMQQueue) startConsumers() error {
	mq.mu.RLock()
	publishCh := NewChannelPublisher(mq.publishCh)
	consumeCh := mq.consumeCh
	handlers := mq.handlers
	mq.mu.RUnlock()

	workerPool := &WorkerPool{
		config: mq.config,
		logger: mq.logger,
		pools:  make(map[string]*WorkerGroup),
	}

	for queueName, reg := range handlers {
		if err := workerPool.StartWorkerGroup(
			queueName,
			reg.workers,
			publishCh,
			consumeCh,
			reg.handler,
		); err != nil {
			return fmt.Errorf("failed to start workers for %s: %w", queueName, err)
		}
	}

	mq.mu.Lock()
	mq.workerPool = workerPool
	mq.mu.Unlock()
	return nil
}

// restartConsumers tears down the old worker pool (whose delivery channels
// died with the previous connection) and starts consumers on the fresh
// channels. Called by the connection monitor after a successful reconnect.
func (mq *RabbitMQQueue) restartConsumers() error {
	mq.mu.RLock()
	closing := mq.closing
	oldPool := mq.workerPool
	registered := len(mq.handlers) > 0
	mq.mu.RUnlock()

	if closing || !registered {
		return nil
	}

	if oldPool != nil {
		// Workers exit promptly: their delivery channels closed with the
		// old connection.
		oldPool.Shutdown(5 * time.Second)
	}

	if err := mq.startConsumers(); err != nil {
		return err
	}

	mq.logger.Info(traceIDRabbitMQHealth, "Consumers restarted after reconnect", nil)
	return nil
}

func (mq *RabbitMQQueue) Shutdown(timeout time.Duration) error {
	mq.logger.Info(traceIDRabbitMQClose, "Shutting down RabbitMQ queue...", nil)

	// Signal the connection monitor to stop before closing the connection so
	// a graceful close is never mistaken for a connection failure.
	mq.mu.Lock()
	if !mq.closing {
		mq.closing = true
		close(mq.stopCh)
	}
	mq.mu.Unlock()

	if mq.workerPool != nil {
		mq.workerPool.Shutdown(timeout)
	}

	mq.mu.Lock()
	defer mq.mu.Unlock()

	if mq.consumeCh != nil {
		if err := mq.consumeCh.Close(); err != nil {
			mq.logger.Error(traceIDRabbitMQClose, "Failed to close channel", nil, customLog.Error(err))
		}
	}

	if mq.conn != nil {
		if err := mq.conn.Close(); err != nil {
			mq.logger.Error(traceIDRabbitMQClose, "Failed to close connection", nil, customLog.Error(err))
		}
	}

	mq.healthy = false
	mq.logger.Info(traceIDRabbitMQClose, "RabbitMQ queue shut down successfully", nil)
	return nil
}
