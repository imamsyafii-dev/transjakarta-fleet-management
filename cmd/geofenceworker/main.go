package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rabbitmq/amqp091-go"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/geofence"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/rabbitmq"
)

const defaultRabbitMQURL = "amqp://fleet:fleet_local_password@127.0.0.1:5672/"

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	consumer, deliveries, err := rabbitmq.OpenConsumer(rabbitMQURL())
	if err != nil {
		log.Fatalf("start geofence worker: %v", err)
	}
	defer func() {
		if err := consumer.Close(); err != nil {
			log.Printf("close RabbitMQ consumer: %v", err)
		}
	}()

	log.Printf("geofence worker consuming queue=%s", rabbitmq.Queue)

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown signal received")
			return
		case delivery, ok := <-deliveries:
			if !ok {
				log.Println("RabbitMQ delivery channel closed")
				return
			}

			processDelivery(delivery)
		}
	}
}

func processDelivery(delivery amqp091.Delivery) {
	event, err := geofence.DecodeEvent(delivery.Body)
	if err != nil {
		log.Printf("discard invalid geofence event: %v", err)
		if err := delivery.Ack(false); err != nil {
			log.Printf("ack invalid geofence event: %v", err)
		}
		return
	}

	log.Printf(
		"processed geofence event vehicle_id=%s event=%s latitude=%f longitude=%f timestamp=%d",
		event.VehicleID,
		event.Event,
		event.Location.Latitude,
		event.Location.Longitude,
		event.Timestamp,
	)

	if err := delivery.Ack(false); err != nil {
		log.Printf("ack geofence event: %v", err)
	}
}

func rabbitMQURL() string {
	if value := strings.TrimSpace(os.Getenv("RABBITMQ_URL")); value != "" {
		return value
	}

	return defaultRabbitMQURL
}
