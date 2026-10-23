package locationwriter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

type recordingSaver struct {
	mu     sync.Mutex
	values []location.Location
}

func (saver *recordingSaver) Save(
	_ context.Context,
	value location.Location,
) error {
	saver.mu.Lock()
	defer saver.mu.Unlock()

	saver.values = append(saver.values, value)
	return nil
}

func (saver *recordingSaver) Values() []location.Location {
	saver.mu.Lock()
	defer saver.mu.Unlock()

	return append([]location.Location(nil), saver.values...)
}

type blockingSaver struct {
	started chan struct{}
	release chan struct{}
}

func (saver *blockingSaver) Save(
	_ context.Context,
	_ location.Location,
) error {
	select {
	case saver.started <- struct{}{}:
	default:
	}

	<-saver.release
	return nil
}

type timeoutSaver struct {
	result chan error
}

func (saver *timeoutSaver) Save(
	ctx context.Context,
	_ location.Location,
) error {
	<-ctx.Done()
	saver.result <- ctx.Err()

	return ctx.Err()
}

type failingFirstSaver struct {
	mu     sync.Mutex
	calls  int
	values []location.Location
}

func (saver *failingFirstSaver) Save(
	_ context.Context,
	value location.Location,
) error {
	saver.mu.Lock()
	defer saver.mu.Unlock()

	saver.calls++
	saver.values = append(saver.values, value)

	if saver.calls == 1 {
		return errors.New("temporary database error")
	}

	return nil
}

func sampleLocation(timestamp int64) location.Location {
	return location.Location{
		VehicleID: "B1234XYZ",
		Latitude:  -6.2088,
		Longitude: 106.8456,
		Timestamp: timestamp,
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	validSaver := &recordingSaver{}

	testCases := []struct {
		name   string
		saver  Saver
		config Config
	}{
		{
			name: "nil saver",
			config: Config{
				QueueSize:   1,
				SaveTimeout: time.Second,
			},
		},
		{
			name:  "zero queue size",
			saver: validSaver,
			config: Config{
				SaveTimeout: time.Second,
			},
		},
		{
			name:  "zero save timeout",
			saver: validSaver,
			config: Config{
				QueueSize: 1,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writer, err := New(testCase.saver, testCase.config)

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if writer != nil {
				t.Fatal("expected nil writer")
			}
		})
	}
}

func TestWriterDrainsLocationsInSubmissionOrder(t *testing.T) {
	saver := &recordingSaver{}

	writer, err := New(saver, Config{
		QueueSize:   3,
		SaveTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	want := []location.Location{
		sampleLocation(1),
		sampleLocation(2),
		sampleLocation(3),
	}

	for _, value := range want {
		if err := writer.Submit(value); err != nil {
			t.Fatalf("submit location: %v", err)
		}
	}

	writer.CloseAndWait()

	got := saver.Values()

	if len(got) != len(want) {
		t.Fatalf("saved locations = %d, want %d", len(got), len(want))
	}

	for index := range want {
		if got[index] != want[index] {
			t.Fatalf(
				"saved location at index %d = %#v, want %#v",
				index,
				got[index],
				want[index],
			)
		}
	}

	err = writer.Submit(sampleLocation(4))
	if !errors.Is(err, ErrClosed) {
		t.Fatalf("submit after close error = %v, want %v", err, ErrClosed)
	}
}

func TestWriterReturnsQueueFull(t *testing.T) {
	saver := &blockingSaver{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}

	writer, err := New(saver, Config{
		QueueSize:   1,
		SaveTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if err := writer.Submit(sampleLocation(1)); err != nil {
		t.Fatalf("submit first location: %v", err)
	}

	select {
	case <-saver.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start saving first location")
	}

	if err := writer.Submit(sampleLocation(2)); err != nil {
		t.Fatalf("submit second location: %v", err)
	}

	err = writer.Submit(sampleLocation(3))
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("third submit error = %v, want %v", err, ErrQueueFull)
	}

	close(saver.release)
	writer.CloseAndWait()
}

func TestWriterContinuesAfterSaveError(t *testing.T) {
	saver := &failingFirstSaver{}

	writer, err := New(saver, Config{
		QueueSize:   2,
		SaveTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if err := writer.Submit(sampleLocation(1)); err != nil {
		t.Fatalf("submit first location: %v", err)
	}

	if err := writer.Submit(sampleLocation(2)); err != nil {
		t.Fatalf("submit second location: %v", err)
	}

	writer.CloseAndWait()

	if saver.calls != 2 {
		t.Fatalf("save calls = %d, want 2", saver.calls)
	}
}

func TestWriterUsesSaveTimeout(t *testing.T) {
	saver := &timeoutSaver{
		result: make(chan error, 1),
	}

	writer, err := New(saver, Config{
		QueueSize:   1,
		SaveTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	if err := writer.Submit(sampleLocation(1)); err != nil {
		t.Fatalf("submit location: %v", err)
	}

	writer.CloseAndWait()

	select {
	case err := <-saver.result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("save context error = %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("saver did not receive timeout context")
	}
}

func TestWriterSubmitConcurrentWithClose(t *testing.T) {
	saver := &recordingSaver{}

	writer, err := New(saver, Config{
		QueueSize:   20,
		SaveTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("create writer: %v", err)
	}

	var submitters sync.WaitGroup
	start := make(chan struct{})

	for index := 0; index < 10; index++ {
		submitters.Add(1)

		go func(timestamp int64) {
			defer submitters.Done()

			<-start
			_ = writer.Submit(sampleLocation(timestamp))
		}(int64(index))
	}

	close(start)

	done := make(chan struct{})

	go func() {
		writer.CloseAndWait()
		close(done)
	}()

	submitters.Wait()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("writer did not finish shutting down")
	}
}
