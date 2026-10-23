package locationwriter

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

var (
	ErrQueueFull = errors.New("location writer queue is full")
	ErrClosed    = errors.New("location writer is closed")
)

type Saver interface {
	Save(context.Context, location.Location) error
}

type Config struct {
	QueueSize   int
	SaveTimeout time.Duration
	OnSaved     func(location.Location)
}

type Writer struct {
	saver       Saver
	jobs        chan location.Location
	saveTimeout time.Duration
	onSaved     func(location.Location)

	mu        sync.RWMutex
	accepting bool
	closeOnce sync.Once
	done      chan struct{}
}

func New(saver Saver, config Config) (*Writer, error) {
	if saver == nil {
		return nil, errors.New("location writer saver must not be nil")
	}

	if config.QueueSize <= 0 {
		return nil, errors.New("location writer queue size must be greater than zero")
	}

	if config.SaveTimeout <= 0 {
		return nil, errors.New("location writer save timeout must be greater than zero")
	}

	writer := &Writer{
		saver:       saver,
		jobs:        make(chan location.Location, config.QueueSize),
		saveTimeout: config.SaveTimeout,
		onSaved:     config.OnSaved,
		accepting:   true,
		done:        make(chan struct{}),
	}

	go writer.run()

	return writer, nil
}

func (writer *Writer) Submit(value location.Location) error {
	if writer == nil {
		return ErrClosed
	}

	writer.mu.RLock()
	defer writer.mu.RUnlock()

	if !writer.accepting {
		return ErrClosed
	}

	select {
	case writer.jobs <- value:
		return nil
	default:
		return ErrQueueFull
	}
}

func (writer *Writer) CloseAndWait() {
	if writer == nil {
		return
	}

	writer.closeOnce.Do(func() {
		writer.mu.Lock()
		writer.accepting = false
		close(writer.jobs)
		writer.mu.Unlock()
	})

	<-writer.done
}

func (writer *Writer) run() {
	defer close(writer.done)

	for value := range writer.jobs {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			writer.saveTimeout,
		)

		err := writer.saver.Save(ctx, value)
		cancel()

		if err != nil {
			log.Printf(
				"failed to save location vehicle_id=%s: %v",
				value.VehicleID,
				err,
			)
			continue
		}

		writer.notifySaved(value)
	}
}

func (writer *Writer) notifySaved(value location.Location) {
	if writer.onSaved == nil {
		return
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			log.Printf(
				"location writer OnSaved panicked vehicle_id=%s: %v",
				value.VehicleID,
				recovered,
			)
		}
	}()

	writer.onSaved(value)
}
