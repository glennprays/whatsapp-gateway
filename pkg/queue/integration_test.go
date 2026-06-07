//go:build integration

package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcrabbitmq "github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	"github.com/glennprays/whatsapp-gateway/config"
)

// rabbitContainer wraps a RabbitMQ testcontainer for integration tests.
type rabbitContainer struct {
	container *tcrabbitmq.RabbitMQContainer
	url       string
}

// startRabbitMQ starts a RabbitMQ container and returns its AMQP URL.
// The container is terminated automatically when the test finishes.
func startRabbitMQ(t *testing.T) *rabbitContainer {
	t.Helper()
	ctx := context.Background()

	container, err := tcrabbitmq.Run(ctx, "rabbitmq:3.13-management-alpine")
	require.NoError(t, err, "failed to start RabbitMQ container")
	t.Cleanup(func() {
		_ = testcontainers.TerminateContainer(container)
	})

	url, err := container.AmqpURL(ctx)
	require.NoError(t, err, "failed to get AMQP URL")

	return &rabbitContainer{container: container, url: url}
}

// stop stops the broker without removing the container (port mapping is
// preserved), simulating a broker outage.
func (rc *rabbitContainer) stop(t *testing.T) {
	t.Helper()
	timeout := 10 * time.Second
	require.NoError(t, rc.container.Stop(context.Background(), &timeout))
}

// start restarts a previously stopped broker on the same mapped port.
func (rc *rabbitContainer) start(t *testing.T) {
	t.Helper()
	require.NoError(t, rc.container.Start(context.Background()))
}

func newTestConfig(url string) *config.Config {
	return &config.Config{
		RabbitMQEnabled:               true,
		RabbitMQURL:                   url,
		RabbitMQConnectionName:        "whatsapp-gateway-test",
		RabbitMQPrefetchCount:         5,
		RabbitMQReconnectDelaySeconds: 1,
		WorkerIncomingEvents:          1,
		WorkerWebhookDelivery:         1,
		WorkerOutgoingMessages:        1,
		QueueMaxRetries:               3,
	}
}

func newTestQueue(t *testing.T, url string) *RabbitMQQueue {
	t.Helper()
	mq, err := NewRabbitMQQueue(newTestConfig(url), newTestLogger())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = mq.Shutdown(5 * time.Second)
	})
	return mq
}

// noopHandler returns a MessageHandler that signals receipt on ch.
func signalHandler(ch chan<- []byte) MessageHandler {
	return func(ctx context.Context, body []byte, headers amqp.Table) error {
		ch <- body
		return nil
	}
}

func discardHandler(ctx context.Context, body []byte, headers amqp.Table) error {
	return nil
}

// waitForBody asserts a message arrives on ch within timeout.
func waitForBody(t *testing.T, ch <-chan []byte, timeout time.Duration) []byte {
	t.Helper()
	select {
	case body := <-ch:
		return body
	case <-time.After(timeout):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func TestIntegration_PublishConsumeRoundTrip(t *testing.T) {
	rc := startRabbitMQ(t)
	mq := newTestQueue(t, rc.url)

	received := make(chan []byte, 1)
	require.NoError(t, mq.StartWorkers(discardHandler, discardHandler, signalHandler(received)))

	err := mq.publish(context.Background(), RoutingKeyOutgoingMsg, map[string]string{"hello": "world"})
	require.NoError(t, err)

	body := waitForBody(t, received, 10*time.Second)
	require.Contains(t, string(body), "hello")
}

func TestIntegration_ConsumersRestartAfterBrokerRestart(t *testing.T) {
	rc := startRabbitMQ(t)
	mq := newTestQueue(t, rc.url)

	received := make(chan []byte, 4)
	require.NoError(t, mq.StartWorkers(discardHandler, discardHandler, signalHandler(received)))

	// Sanity: round trip works before the outage.
	require.NoError(t, mq.publish(context.Background(), RoutingKeyOutgoingMsg, map[string]string{"phase": "before"}))
	waitForBody(t, received, 10*time.Second)

	// Simulate a broker outage and recovery.
	rc.stop(t)
	require.Eventually(t, func() bool { return !mq.IsHealthy() }, 15*time.Second, 200*time.Millisecond,
		"queue must report unhealthy while broker is down")

	rc.start(t)
	require.Eventually(t, func() bool { return mq.IsHealthy() }, 60*time.Second, 500*time.Millisecond,
		"queue must reconnect after broker comes back")

	// The critical assertion: consuming must resume after reconnect.
	require.Eventually(t, func() bool {
		if err := mq.publish(context.Background(), RoutingKeyOutgoingMsg, map[string]string{"phase": "after"}); err != nil {
			return false
		}
		select {
		case <-received:
			return true
		case <-time.After(2 * time.Second):
			return false
		}
	}, 30*time.Second, time.Second, "messages published after broker restart must be consumed")
}

func TestIntegration_FailedMessageRetriesViaTierQueue(t *testing.T) {
	rc := startRabbitMQ(t)
	mq := newTestQueue(t, rc.url)

	var mu sync.Mutex
	attempts := 0
	received := make(chan []byte, 1)
	handler := func(ctx context.Context, body []byte, headers amqp.Table) error {
		mu.Lock()
		attempts++
		n := attempts
		mu.Unlock()
		if n == 1 {
			return errors.New("transient failure")
		}
		received <- body
		return nil
	}
	require.NoError(t, mq.StartWorkers(discardHandler, discardHandler, handler))

	start := time.Now()
	require.NoError(t, mq.publish(context.Background(), RoutingKeyOutgoingMsg, map[string]string{"n": "1"}))

	// First attempt fails; redelivery flows through the 1s tier queue.
	waitForBody(t, received, 15*time.Second)
	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "retry must be delayed by the 1s tier TTL")

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, attempts)
}

func TestIntegration_GracefulShutdownStopsMonitor(t *testing.T) {
	rc := startRabbitMQ(t)
	mq, err := NewRabbitMQQueue(newTestConfig(rc.url), newTestLogger())
	require.NoError(t, err)

	require.NoError(t, mq.Shutdown(5*time.Second))
	require.False(t, mq.IsHealthy())

	// Give the monitor goroutine a moment; it must exit without attempting
	// to reconnect (reconnect would flip healthy back to true).
	time.Sleep(2 * time.Second)
	require.False(t, mq.IsHealthy(), "monitor must not reconnect after graceful shutdown")
}
