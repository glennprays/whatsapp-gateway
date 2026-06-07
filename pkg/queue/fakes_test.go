package queue

import (
	"sync"

	customLog "github.com/glennprays/log"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/glennprays/whatsapp-gateway/config"
)

// fakeAcknowledger records Ack/Nack calls made through amqp.Delivery.
type fakeAcknowledger struct {
	mu       sync.Mutex
	acks     int
	nacks    []bool // requeue flag per nack
	ackErr   error
	nackErr  error
	rejected int
}

func (f *fakeAcknowledger) Ack(tag uint64, multiple bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acks++
	return f.ackErr
}

func (f *fakeAcknowledger) Nack(tag uint64, multiple bool, requeue bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nacks = append(f.nacks, requeue)
	return f.nackErr
}

func (f *fakeAcknowledger) Reject(tag uint64, requeue bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rejected++
	return nil
}

// fakePublisher records publishes made by publishRetry.
type fakePublisher struct {
	mu         sync.Mutex
	published  []amqp.Publishing
	routingKey []string
	err        error
}

func (f *fakePublisher) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.published = append(f.published, msg)
	f.routingKey = append(f.routingKey, key)
	return f.err
}

func newTestLogger() *customLog.Logger {
	logger, err := customLog.New(customLog.Config{
		Service: "queue-test",
		Env:     "dev",
		Level:   customLog.ErrorLevel,
		Output:  customLog.OutputStdout,
	})
	if err != nil {
		panic(err)
	}
	return logger
}

func newTestWorkerGroup(publisher AMQPPublisher, maxRetries int, handler MessageHandler) *WorkerGroup {
	return &WorkerGroup{
		queueName: QueueOutgoingMessages,
		workers:   1,
		publishCh: publisher,
		handler:   handler,
		logger:    newTestLogger(),
		config:    &config.Config{QueueMaxRetries: maxRetries},
		stopChan:  make(chan struct{}),
	}
}
