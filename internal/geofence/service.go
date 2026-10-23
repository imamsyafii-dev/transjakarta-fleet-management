package geofence

import (
	"context"
	"errors"
	"time"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

type Publisher interface {
	Publish(context.Context, Event) error
}

type Service struct {
	detector       *Detector
	publisher      Publisher
	publishTimeout time.Duration
}

func NewService(
	detector *Detector,
	publisher Publisher,
	publishTimeout time.Duration,
) (*Service, error) {
	if detector == nil {
		return nil, errors.New("geofence detector must not be nil")
	}
	if publisher == nil {
		return nil, errors.New("geofence publisher must not be nil")
	}
	if publishTimeout <= 0 {
		return nil, errors.New("geofence publish timeout must be greater than zero")
	}

	return &Service{
		detector:       detector,
		publisher:      publisher,
		publishTimeout: publishTimeout,
	}, nil
}

func (service *Service) Process(value location.Location) error {
	if service == nil {
		return errors.New("geofence service is not initialized")
	}

	observation, err := service.detector.Observe(value)
	if err != nil {
		return err
	}
	if !observation.Entered {
		return nil
	}

	event, err := NewEntryEvent(value)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		service.publishTimeout,
	)
	defer cancel()

	return service.publisher.Publish(ctx, event)
}
