package queue

import (
	"context"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func newDelivery(ack *fakeAcknowledger, headers amqp.Table) amqp.Delivery {
	return amqp.Delivery{
		Acknowledger: ack,
		DeliveryTag:  1,
		Headers:      headers,
		Body:         []byte(`{}`),
	}
}

func TestProcessMessage_SuccessAcks(t *testing.T) {
	ack := &fakeAcknowledger{}
	wg := newTestWorkerGroup(&fakePublisher{}, 3, func(ctx context.Context, body []byte, headers amqp.Table) error {
		return nil
	})

	wg.processMessage("test-worker", newDelivery(ack, nil))

	if ack.acks != 1 {
		t.Fatalf("expected 1 ack, got %d", ack.acks)
	}
	if len(ack.nacks) != 0 {
		t.Fatalf("expected no nacks, got %d", len(ack.nacks))
	}
}

func TestProcessMessage_ErrorSchedulesRetryAndAcks(t *testing.T) {
	ack := &fakeAcknowledger{}
	pub := &fakePublisher{}
	wg := newTestWorkerGroup(pub, 3, func(ctx context.Context, body []byte, headers amqp.Table) error {
		return errors.New("handler failed")
	})

	wg.processMessage("test-worker", newDelivery(ack, nil))

	if len(pub.published) != 1 {
		t.Fatalf("expected 1 retry publish, got %d", len(pub.published))
	}
	if pub.routingKey[0] != QueueOutgoingMessages+".retry" {
		t.Fatalf("unexpected retry routing key: %s", pub.routingKey[0])
	}
	if pub.published[0].Headers[RetryCountKey] != 1 {
		t.Fatalf("expected retry count 1, got %v", pub.published[0].Headers[RetryCountKey])
	}
	if pub.published[0].Expiration == "" {
		t.Fatal("expected expiration to be set on retry message")
	}
	if ack.acks != 1 {
		t.Fatalf("expected original message acked after retry scheduled, got %d acks", ack.acks)
	}
}

func TestProcessMessage_MaxRetriesNacksToDLQ(t *testing.T) {
	ack := &fakeAcknowledger{}
	pub := &fakePublisher{}
	wg := newTestWorkerGroup(pub, 3, func(ctx context.Context, body []byte, headers amqp.Table) error {
		return errors.New("handler failed")
	})

	wg.processMessage("test-worker", newDelivery(ack, amqp.Table{RetryCountKey: int32(3)}))

	if len(pub.published) != 0 {
		t.Fatalf("expected no retry publish at max retries, got %d", len(pub.published))
	}
	if len(ack.nacks) != 1 {
		t.Fatalf("expected 1 nack, got %d", len(ack.nacks))
	}
	if requeue := ack.nacks[0]; requeue {
		t.Fatal("expected nack without requeue (route to DLQ)")
	}
}

func TestProcessMessage_RetryPublishFailureNacksWithRequeue(t *testing.T) {
	ack := &fakeAcknowledger{}
	pub := &fakePublisher{err: errors.New("broker unavailable")}
	wg := newTestWorkerGroup(pub, 3, func(ctx context.Context, body []byte, headers amqp.Table) error {
		return errors.New("handler failed")
	})

	wg.processMessage("test-worker", newDelivery(ack, nil))

	if ack.acks != 0 {
		t.Fatalf("expected no ack when retry publish fails, got %d", ack.acks)
	}
	if len(ack.nacks) != 1 {
		t.Fatalf("expected 1 nack, got %d", len(ack.nacks))
	}
	if requeue := ack.nacks[0]; !requeue {
		t.Fatal("expected nack with requeue when retry publish fails")
	}
}

func TestProcessMessage_PanicRecoversAndNacksToDLQ(t *testing.T) {
	ack := &fakeAcknowledger{}
	wg := newTestWorkerGroup(&fakePublisher{}, 3, func(ctx context.Context, body []byte, headers amqp.Table) error {
		panic("handler exploded")
	})

	// Must not propagate the panic.
	wg.processMessage("test-worker", newDelivery(ack, nil))

	if len(ack.nacks) != 1 {
		t.Fatalf("expected 1 nack after panic, got %d", len(ack.nacks))
	}
	if requeue := ack.nacks[0]; requeue {
		t.Fatal("expected nack without requeue (route to DLQ) after panic")
	}
}

func TestProcessMessage_AckErrorIsHandled(t *testing.T) {
	ack := &fakeAcknowledger{ackErr: errors.New("channel closed")}
	wg := newTestWorkerGroup(&fakePublisher{}, 3, func(ctx context.Context, body []byte, headers amqp.Table) error {
		return nil
	})

	// Must not panic; error is logged.
	wg.processMessage("test-worker", newDelivery(ack, nil))

	if ack.acks != 1 {
		t.Fatalf("expected ack attempted once, got %d", ack.acks)
	}
}

func TestRetryBackoff(t *testing.T) {
	cases := []struct {
		retry int
		want  string
	}{
		{0, "1s"}, {1, "5s"}, {2, "25s"}, {3, "25s"}, {99, "25s"},
	}
	for _, c := range cases {
		if got := retryBackoff(c.retry).String(); got != c.want {
			t.Errorf("retryBackoff(%d) = %s, want %s", c.retry, got, c.want)
		}
	}
}

func TestGetRetryCount(t *testing.T) {
	if got := getRetryCount(nil); got != 0 {
		t.Errorf("nil headers: got %d, want 0", got)
	}
	if got := getRetryCount(amqp.Table{RetryCountKey: int64(2)}); got != 2 {
		t.Errorf("int64 header: got %d, want 2", got)
	}
	if got := getRetryCount(amqp.Table{RetryCountKey: "bogus"}); got != 0 {
		t.Errorf("non-numeric header: got %d, want 0", got)
	}
}
