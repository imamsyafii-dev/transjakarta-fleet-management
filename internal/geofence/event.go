package geofence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

const EntryEvent = "geofence_entry"

type EventLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Event struct {
	VehicleID string        `json:"vehicle_id"`
	Event     string        `json:"event"`
	Location  EventLocation `json:"location"`
	Timestamp int64         `json:"timestamp"`
}

func DecodeEvent(payload []byte) (Event, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	var event Event
	if err := decoder.Decode(&event); err != nil {
		return Event{}, fmt.Errorf("decode geofence event: %w", err)
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Event{}, fmt.Errorf("decode geofence event: payload must contain one JSON object")
	}

	if err := event.Validate(); err != nil {
		return Event{}, fmt.Errorf("validate geofence event: %w", err)
	}

	return event, nil
}

func NewEntryEvent(value location.Location) (Event, error) {
	if err := value.Validate(); err != nil {
		return Event{}, fmt.Errorf("validate location for geofence event: %w", err)
	}

	return Event{
		VehicleID: value.VehicleID,
		Event:     EntryEvent,
		Location: EventLocation{
			Latitude:  value.Latitude,
			Longitude: value.Longitude,
		},
		Timestamp: value.Timestamp,
	}, nil
}

func (event Event) Validate() error {
	if event.Event != EntryEvent {
		return fmt.Errorf("event must be %q", EntryEvent)
	}

	return location.Location{
		VehicleID: event.VehicleID,
		Latitude:  event.Location.Latitude,
		Longitude: event.Location.Longitude,
		Timestamp: event.Timestamp,
	}.Validate()
}
