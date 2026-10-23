package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/geofence"
)

const (
	Exchange   = "fleet.events"
	Queue      = "geofence_alerts"
	RoutingKey = "geofence.entry"
)

type Publisher struct {
	connection *amqp091.Connection
	channel    *amqp091.Channel
}

func Open(url string) (*Publisher, error) {
	if strings.TrimSpace(url) == "" {
		return nil, fmt.Errorf("RabbitMQ URL must not be empty")
	}

	connection, err := amqp091.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("connect to RabbitMQ: %w", err)
	}

	channel, err := connection.Channel()
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("open RabbitMQ channel: %w", err)
	}

	if err := DeclareTopology(channel); err != nil {
		_ = channel.Close()
		_ = connection.Close()
		return nil, err
	}

	return &Publisher{
		connection: connection,
		channel:    channel,
	}, nil
}

func (publisher *Publisher) Publish(
	ctx context.Context,
	event geofence.Event,
) error {
	if publisher == nil || publisher.channel == nil {
		return fmt.Errorf("RabbitMQ publisher is not initialized")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("validate geofence event: %w", err)
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode geofence event: %w", err)
	}

	err = publisher.channel.PublishWithContext(
		ctx,
		Exchange,
		RoutingKey,
		false,
		false,
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Timestamp:    time.Now().UTC(),
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("publish geofence event: %w", err)
	}

	return nil
}

func (publisher *Publisher) Close() error {
	if publisher == nil {
		return nil
	}

	var closeErr error
	if publisher.channel != nil {
		closeErr = publisher.channel.Close()
	}
	if publisher.connection != nil {
		if err := publisher.connection.Close(); closeErr == nil {
			closeErr = err
		}
	}

	return closeErr
}

// DeclareTopology creates the durable exchange, queue, and binding used by
// geofence events. It is safe to call for both publishers and consumers.
func DeclareTopology(channel *amqp091.Channel) error {
	if err := channel.ExchangeDeclare(
		Exchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare RabbitMQ exchange: %w", err)
	}

	if _, err := channel.QueueDeclare(
		Queue,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare RabbitMQ queue: %w", err)
	}

	if err := channel.QueueBind(
		Queue,
		RoutingKey,
		Exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind RabbitMQ queue: %w", err)
	}

	return nil
}
