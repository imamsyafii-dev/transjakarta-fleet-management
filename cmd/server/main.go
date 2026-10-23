package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/mqttsubscriber"
)

const (
	defaultHTTPAddress = ":8080"
	defaultMQTTBroker  = "tcp://127.0.0.1:1883"
	defaultMQTTClient  = "fleet-server"
)

type config struct {
	httpAddress string
	mqttBroker  string
	mqttClient  string
}

func main() {
	cfg := loadConfig()

	subscriber, err := mqttsubscriber.Connect(
		mqttsubscriber.Config{
			Broker:   cfg.mqttBroker,
			ClientID: cfg.mqttClient,
		},
		logLocation,
	)
	if err != nil {
		log.Fatalf("start MQTT subscriber: %v", err)
	}
	defer subscriber.Disconnect()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	server := &http.Server{
		Addr:              cfg.httpAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErrors := make(chan error, 1)

	go func() {
		log.Printf("HTTP server listening on %s", cfg.httpAddress)

		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")
	case err := <-serverErrors:
		log.Printf("HTTP server stopped unexpectedly: %v", err)
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("shutdown HTTP server: %v", err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status": "ok",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("encode health response: %v", err)
	}
}

func logLocation(location location.Location) {
	log.Printf(
		"received location vehicle_id=%s latitude=%f longitude=%f timestamp=%d",
		location.VehicleID,
		location.Latitude,
		location.Longitude,
		location.Timestamp,
	)
}

func loadConfig() config {
	return config{
		httpAddress: valueOrDefault("HTTP_ADDRESS", defaultHTTPAddress),
		mqttBroker:  valueOrDefault("MQTT_BROKER", defaultMQTTBroker),
		mqttClient:  valueOrDefault("MQTT_CLIENT_ID", defaultMQTTClient),
	}
}

func valueOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return defaultValue
}
