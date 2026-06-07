package queue

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AMQPConfirmation is the subset of *amqp.DeferredConfirmation used to wait
// for a broker publish acknowledgement.
type AMQPConfirmation interface {
	WaitContext(ctx context.Context) (bool, error)
}

// AMQPPublisher is the subset of *amqp.Channel used by worker groups to
// publish messages. Extracted so retry logic can be unit-tested with fakes.
type AMQPPublisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	PublishWithConfirm(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (AMQPConfirmation, error)
}

// channelPublisher adapts *amqp.Channel to AMQPPublisher.
type channelPublisher struct {
	ch *amqp.Channel
}

// NewChannelPublisher wraps an AMQP channel as an AMQPPublisher.
func NewChannelPublisher(ch *amqp.Channel) AMQPPublisher {
	return channelPublisher{ch: ch}
}

func (p channelPublisher) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	return p.ch.Publish(exchange, key, mandatory, immediate, msg)
}

func (p channelPublisher) PublishWithConfirm(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) (AMQPConfirmation, error) {
	conf, err := p.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, key, mandatory, immediate, msg)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

// AMQPConsumer is the subset of *amqp.Channel used by worker groups to
// consume messages.
type AMQPConsumer interface {
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
}
