package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/geofence"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/locationapi"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/locationstore"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/locationwriter"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/mqttsubscriber"
	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/rabbitmq"
)

const (
	defaultHTTPAddress = ":8080"
	defaultMQTTBroker  = "tcp://127.0.0.1:1883"
	defaultMQTTClient  = "fleet-server"
	defaultRabbitMQURL = "amqp://fleet:fleet_local_password@127.0.0.1:5672/"

	databaseConnectTimeout = 5 * time.Second
	locationQueueSize      = 100
	locationSaveTimeout    = 3 * time.Second
	locationQueryTimeout   = 3 * time.Second
	geofenceEventTimeout   = 3 * time.Second
	geofenceLatitude       = -6.2090
	geofenceLongitude      = 106.8456
	geofenceRadiusMeters   = 50.0
)

type config struct {
	httpAddress  string
	mqttBroker   string
	mqttClient   string
	databaseURL  string
	rabbitMQURL  string
	geofence     geofence.Config
	eventTimeout time.Duration
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	if cfg.databaseURL == "" {
		log.Fatal("DATABASE_URL must not be empty")
	}

	databaseContext, cancelDatabase := context.WithTimeout(
		context.Background(),
		databaseConnectTimeout,
	)

	store, err := locationstore.Open(databaseContext, cfg.databaseURL)
	cancelDatabase()

	if err != nil {
		log.Fatalf("open PostgreSQL location store: %v", err)
	}

	locationAPI, err := locationapi.New(store, locationapi.Config{
		QueryTimeout: locationQueryTimeout,
	})
	if err != nil {
		store.Close()
		log.Fatalf("create location API: %v", err)
	}

	rabbitPublisher, err := rabbitmq.Open(cfg.rabbitMQURL)
	if err != nil {
		store.Close()
		log.Fatalf("open RabbitMQ publisher: %v", err)
	}

	detector, err := geofence.New(cfg.geofence)
	if err != nil {
		_ = rabbitPublisher.Close()
		store.Close()
		log.Fatalf("create geofence detector: %v", err)
	}

	geofenceService, err := geofence.NewService(
		detector,
		rabbitPublisher,
		cfg.eventTimeout,
	)
	if err != nil {
		_ = rabbitPublisher.Close()
		store.Close()
		log.Fatalf("create geofence service: %v", err)
	}

	writer, err := locationwriter.New(store, locationwriter.Config{
		QueueSize:   locationQueueSize,
		SaveTimeout: locationSaveTimeout,
		OnSaved: func(value location.Location) {
			if err := geofenceService.Process(value); err != nil {
				log.Printf(
					"process geofence vehicle_id=%s: %v",
					value.VehicleID,
					err,
				)
			}
		},
	})
	if err != nil {
		_ = rabbitPublisher.Close()
		store.Close()
		log.Fatalf("create location writer: %v", err)
	}

	subscriber, err := mqttsubscriber.Connect(
		mqttsubscriber.Config{
			Broker:   cfg.mqttBroker,
			ClientID: cfg.mqttClient,
		},
		handleLocation(writer),
	)
	if err != nil {
		writer.CloseAndWait()
		_ = rabbitPublisher.Close()
		store.Close()
		log.Fatalf("start MQTT subscriber: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", healthHandler)

	locationAPI.Register(router)

	server := &http.Server{
		Addr:              cfg.httpAddress,
		Handler:           router,
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

	subscriber.Disconnect()

	shutdownContext, cancelShutdown := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownContext); err != nil {
		log.Printf("shutdown HTTP server: %v", err)
	}

	writer.CloseAndWait()
	if err := rabbitPublisher.Close(); err != nil {
		log.Printf("close RabbitMQ publisher: %v", err)
	}
	store.Close()
}

func handleLocation(
	writer *locationwriter.Writer,
) mqttsubscriber.LocationHandler {
	return func(value location.Location) {
		logLocation(value)

		if err := writer.Submit(value); err != nil {
			log.Printf(
				"queue location vehicle_id=%s: %v",
				value.VehicleID,
				err,
			)
		}
	}
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

func loadConfig() (config, error) {
	latitude, err := floatValueOrDefault("GEOFENCE_LATITUDE", geofenceLatitude)
	if err != nil {
		return config{}, err
	}

	longitude, err := floatValueOrDefault("GEOFENCE_LONGITUDE", geofenceLongitude)
	if err != nil {
		return config{}, err
	}

	radiusMeters, err := floatValueOrDefault(
		"GEOFENCE_RADIUS_METERS",
		geofenceRadiusMeters,
	)
	if err != nil {
		return config{}, err
	}

	eventTimeout, err := durationValueOrDefault(
		"GEOFENCE_EVENT_TIMEOUT",
		geofenceEventTimeout,
	)
	if err != nil {
		return config{}, err
	}

	return config{
		httpAddress: valueOrDefault("HTTP_ADDRESS", defaultHTTPAddress),
		mqttBroker:  valueOrDefault("MQTT_BROKER", defaultMQTTBroker),
		mqttClient:  valueOrDefault("MQTT_CLIENT_ID", defaultMQTTClient),
		databaseURL: valueOrDefault("DATABASE_URL", ""),
		rabbitMQURL: valueOrDefault("RABBITMQ_URL", defaultRabbitMQURL),
		geofence: geofence.Config{
			CenterLatitude:  latitude,
			CenterLongitude: longitude,
			RadiusMeters:    radiusMeters,
		},
		eventTimeout: eventTimeout,
	}, nil
}

func valueOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return defaultValue
}

func floatValueOrDefault(key string, defaultValue float64) (float64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return parsed, nil
}

func durationValueOrDefault(
	key string,
	defaultValue time.Duration,
) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}

	return parsed, nil
}
