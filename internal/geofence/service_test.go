package geofence

import (
	"context"
	"testing"
	"time"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

type recordingPublisher struct {
	events []Event
}

func (publisher *recordingPublisher) Publish(
	_ context.Context,
	event Event,
) error {
	publisher.events = append(publisher.events, event)
	return nil
}

func TestNewEntryEvent(t *testing.T) {
	value := location.Location{
		VehicleID: "B1234XYZ",
		Latitude:  -6.2088,
		Longitude: 106.8456,
		Timestamp: 1715003456,
	}

	event, err := NewEntryEvent(value)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}

	if event.VehicleID != value.VehicleID ||
		event.Event != EntryEvent ||
		event.Location.Latitude != value.Latitude ||
		event.Location.Longitude != value.Longitude ||
		event.Timestamp != value.Timestamp {
		t.Fatalf("event = %#v, does not match source location", event)
	}
}

func TestServicePublishesOnlyOnEntry(t *testing.T) {
	detector, err := New(Config{RadiusMeters: 20})
	if err != nil {
		t.Fatalf("create detector: %v", err)
	}

	publisher := &recordingPublisher{}
	service, err := NewService(detector, publisher, time.Second)
	if err != nil {
		t.Fatalf("create service: %v", err)
	}

	locations := []location.Location{
		sampleLocation("B1234XYZ", 0.0010),
		sampleLocation("B1234XYZ", 0.0001),
		sampleLocation("B1234XYZ", 0.0001),
		sampleLocation("B1234XYZ", 0.0010),
		sampleLocation("B1234XYZ", 0.0001),
	}

	for index, value := range locations {
		if err := service.Process(value); err != nil {
			t.Fatalf("process location at index %d: %v", index, err)
		}
	}

	if len(publisher.events) != 2 {
		t.Fatalf("published events = %d, want 2", len(publisher.events))
	}

	for _, event := range publisher.events {
		if err := event.Validate(); err != nil {
			t.Fatalf("published event is invalid: %v", err)
		}
	}
}
