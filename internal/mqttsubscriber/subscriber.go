package mqttsubscriber

import (
	"fmt"
	"log"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

const LocationTopic = "/fleet/vehicle/+/location"

type Config struct {
	Broker   string
	ClientID string
}

// LocationHandler must return quickly because it runs in a Paho callback.
type LocationHandler func(location.Location)

type Subscriber struct {
	client mqtt.Client
}

func Connect(config Config, onLocation LocationHandler) (*Subscriber, error) {
	if strings.TrimSpace(config.Broker) == "" {
		return nil, fmt.Errorf("MQTT broker must not be empty")
	}
	if strings.TrimSpace(config.ClientID) == "" {
		return nil, fmt.Errorf("MQTT client ID must not be empty")
	}
	if onLocation == nil {
		return nil, fmt.Errorf("location handler must not be nil")
	}

	messageHandler := func(_ mqtt.Client, message mqtt.Message) {
		handleMessage(message, onLocation)
	}

	options := mqtt.NewClientOptions().
		AddBroker(config.Broker).
		SetClientID(config.ClientID).
		SetConnectTimeout(5 * time.Second).
		SetAutoReconnect(true).
		SetResumeSubs(true).
		SetCleanSession(false).
		SetDefaultPublishHandler(messageHandler)

	client := mqtt.NewClient(options)

	connectToken := client.Connect()
	if !connectToken.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("connect to MQTT broker timed out")
	}
	if err := connectToken.Error(); err != nil {
		return nil, fmt.Errorf("connect to MQTT broker: %w", err)
	}

	subscribeToken := client.Subscribe(LocationTopic, 1, nil)

	if !subscribeToken.WaitTimeout(5 * time.Second) {
		client.Disconnect(250)
		return nil, fmt.Errorf("subscribe to location topic timed out")
	}
	if err := subscribeToken.Error(); err != nil {
		client.Disconnect(250)
		return nil, fmt.Errorf("subscribe to location topic: %w", err)
	}

	log.Printf(
		"subscribed broker=%s client_id=%s topic=%s",
		config.Broker,
		config.ClientID,
		LocationTopic,
	)

	return &Subscriber{client: client}, nil
}

func (subscriber *Subscriber) Disconnect() {
	if subscriber != nil && subscriber.client != nil {
		subscriber.client.Disconnect(250)
	}
}

func handleMessage(message mqtt.Message, onLocation LocationHandler) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf(
				"panic while processing MQTT message topic=%q: %v",
				message.Topic(),
				recovered,
			)
		}
	}()

	topicVehicleID, err := location.ParseTopic(message.Topic())
	if err != nil {
		log.Printf("rejected MQTT message topic=%q: %v", message.Topic(), err)
		return
	}

	payload, err := location.Decode(message.Payload())
	if err != nil {
		log.Printf("rejected MQTT message topic=%q: %v", message.Topic(), err)
		return
	}

	if payload.VehicleID != topicVehicleID {
		log.Printf(
			"rejected MQTT message topic=%q: payload vehicle_id %q does not match topic vehicle_id %q",
			message.Topic(),
			payload.VehicleID,
			topicVehicleID,
		)
		return
	}

	onLocation(payload)
}
