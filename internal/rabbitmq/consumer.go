package rabbitmq

import (
	"fmt"
	"strings"

	"github.com/rabbitmq/amqp091-go"
)

type Consumer struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
}

func OpenConsumer(url string) (*Consumer, <-chan amqp091.Delivery, error) {
	if strings.TrimSpace(url) == "" {
		return nil, nil, fmt.Errorf("RabbitMQ URL must not be empty")
	}

	connection, err := amqp091.Dial(url)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	if err := DeclareTopology(channel); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, nil, err
	}
	if err := channel.Qos(1, 0, false); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, nil, fmt.Errorf("configure RabbitMQ consumer QoS: %w", err)
	}

	deliveries, err := channel.Consume(
		Queue,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, nil, fmt.Errorf("consume RabbitMQ queue: %w", err)
	}

	return &Consumer{
		connection: connection,
		channel:    channel,
	}, deliveries, nil
}

func (consumer *Consumer) Close() error {
	if consumer == nil {
		return nil
	}

	var closeErr error
	if consumer.channel != nil {
		closeErr = consumer.channel.Close()
	}
	if consumer.connection != nil {
		if err := consumer.connection.Close(); closeErr == nil {
			closeErr = err
		}
	}

	return closeErr
}
