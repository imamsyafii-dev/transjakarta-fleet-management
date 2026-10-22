package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	defaultBroker   = "tcp://127.0.0.1:1883"
	defaultVehicle  = "B1234XYZ"
	defaultInterval = "2s"
)

type config struct {
	broker   string
	clientID string
	vehicle  string
	interval time.Duration
}

type point struct {
	latitude  float64
	longitude float64
}

type locationPayload struct {
	VehicleID string  `json:"vehicle_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp"`
}

var mockRoute = []point{
	{latitude: -6.2100, longitude: 106.8456},
	{latitude: -6.2094, longitude: 106.8456},
	{latitude: -6.2090, longitude: 106.8456},
	{latitude: -6.2087, longitude: 106.8456},
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	client, err := connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(250)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	log.Printf(
		"connected broker=%s client_id=%s interval=%s",
		cfg.broker,
		cfg.clientID,
		cfg.interval,
	)

	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	for index := 0; ; index++ {
		if err := publishLocation(client, cfg.vehicle, mockRoute[index%len(mockRoute)]); err != nil {
			log.Printf("publish location: %v", err)
		}

		select {
		case <-ctx.Done():
			log.Println("publisher stopped")
			return
		case <-ticker.C:
		}
	}
}

func loadConfig() (config, error) {
	interval, err := time.ParseDuration(
		valueOrDefault("PUBLISH_INTERVAL", defaultInterval),
	)
	if err != nil {
		return config{}, fmt.Errorf("parse PUBLISH_INTERVAL: %w", err)
	}
	if interval <= 0 {
		return config{}, fmt.Errorf("PUBLISH_INTERVAL must be greater than zero")
	}

	vehicleID := valueOrDefault("VEHICLE_ID", defaultVehicle)

	return config{
		broker:   valueOrDefault("MQTT_BROKER", defaultBroker),
		clientID: valueOrDefault("MQTT_CLIENT_ID", "fleet-publisher-"+vehicleID),
		vehicle:  vehicleID,
		interval: interval,
	}, nil
}

func connect(cfg config) (mqtt.Client, error) {
	options := mqtt.NewClientOptions().
		AddBroker(cfg.broker).
		SetClientID(cfg.clientID).
		SetConnectTimeout(5 * time.Second).
		SetAutoReconnect(true)

	client := mqtt.NewClient(options)

	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		return nil, fmt.Errorf("connect to MQTT broker timed out")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("connect to MQTT broker: %w", err)
	}

	return client, nil
}

func publishLocation(client mqtt.Client, vehicleID string, point point) error {
	payload := locationPayload{
		VehicleID: vehicleID,
		Latitude:  point.latitude,
		Longitude: point.longitude,
		Timestamp: time.Now().Unix(),
	}

	message, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode location payload: %w", err)
	}

	topic := locationTopic(vehicleID)

	token := client.Publish(topic, 1, false, message)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timed out")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("publish location: %w", err)
	}

	log.Printf("published topic=%s payload=%s", topic, message)
	return nil
}

func valueOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return defaultValue
}

func locationTopic(vehicleID string) string {
	return fmt.Sprintf("/fleet/vehicle/%s/location", vehicleID)
}
