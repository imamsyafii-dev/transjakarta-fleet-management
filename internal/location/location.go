package location

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
)

type Location struct {
	VehicleID string  `json:"vehicle_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp"`
}

func Decode(payload []byte) (Location, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()

	var location Location
	if err := decoder.Decode(&location); err != nil {
		return Location{}, fmt.Errorf("decode location payload: %w", err)
	}

	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Location{}, fmt.Errorf("payload must contain exactly one JSON object")
		}

		return Location{}, fmt.Errorf("decode trailing payload data: %w", err)
	}

	if err := location.Validate(); err != nil {
		return Location{}, fmt.Errorf("validate location payload: %w", err)
	}

	return location, nil
}

func (location Location) Validate() error {
	if err := validateVehicleID(location.VehicleID); err != nil {
		return fmt.Errorf("vehicle_id: %w", err)
	}

	if math.IsNaN(location.Latitude) || math.IsInf(location.Latitude, 0) {
		return fmt.Errorf("latitude must be a finite number")
	}
	if location.Latitude < -90 || location.Latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}

	if math.IsNaN(location.Longitude) || math.IsInf(location.Longitude, 0) {
		return fmt.Errorf("longitude must be a finite number")
	}
	if location.Longitude < -180 || location.Longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")
	}

	if location.Timestamp <= 0 {
		return fmt.Errorf("timestamp must be greater than zero")
	}

	return nil
}

func ParseTopic(topic string) (string, error) {
	parts := strings.Split(topic, "/")

	if len(parts) != 5 ||
		parts[0] != "" ||
		parts[1] != "fleet" ||
		parts[2] != "vehicle" ||
		parts[4] != "location" {
		return "", fmt.Errorf("topic must match /fleet/vehicle/{vehicle_id}/location")
	}

	vehicleID := parts[3]
	if err := validateVehicleID(vehicleID); err != nil {
		return "", fmt.Errorf("topic vehicle_id: %w", err)
	}

	return vehicleID, nil
}

func validateVehicleID(vehicleID string) error {
	if strings.TrimSpace(vehicleID) == "" {
		return fmt.Errorf("must not be empty")
	}
	if len(vehicleID) > 64 {
		return fmt.Errorf("must be at most 64 characters")
	}

	for _, character := range vehicleID {
		isLetter := character >= 'A' && character <= 'Z' ||
			character >= 'a' && character <= 'z'
		isNumber := character >= '0' && character <= '9'
		isAllowedSymbol := character == '-' || character == '_'

		if !isLetter && !isNumber && !isAllowedSymbol {
			return fmt.Errorf("contains invalid character %q", character)
		}
	}

	return nil
}
