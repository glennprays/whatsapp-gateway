package queue

import (
	amqp "github.com/rabbitmq/amqp091-go"
)

// AMQPPublisher is the subset of *amqp.Channel used by worker groups to
// publish messages. Extracted so retry logic can be unit-tested with fakes.
type AMQPPublisher interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

// AMQPConsumer is the subset of *amqp.Channel used by worker groups to
// consume messages.
type AMQPConsumer interface {
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
}
