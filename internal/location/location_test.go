package location

import (
	"math"
	"strings"
	"testing"
)

func TestParseTopic(t *testing.T) {
	tests := []struct {
		name    string
		topic   string
		wantID  string
		wantErr bool
	}{
		{
			name:   "valid topic",
			topic:  "/fleet/vehicle/B1234XYZ/location",
			wantID: "B1234XYZ",
		},
		{
			name:    "missing leading slash",
			topic:   "fleet/vehicle/B1234XYZ/location",
			wantErr: true,
		},
		{
			name:    "empty vehicle ID",
			topic:   "/fleet/vehicle//location",
			wantErr: true,
		},
		{
			name:    "invalid vehicle ID character",
			topic:   "/fleet/vehicle/B 123/location",
			wantErr: true,
		},
		{
			name:    "extra topic segment",
			topic:   "/fleet/vehicle/B1234XYZ/location/extra",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			vehicleID, err := ParseTopic(test.topic)

			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseTopic returned error: %v", err)
			}
			if vehicleID != test.wantID {
				t.Fatalf("got vehicle ID %q, want %q", vehicleID, test.wantID)
			}
		})
	}
}

func TestLocationValidate(t *testing.T) {
	validLocation := Location{
		VehicleID: "B1234XYZ",
		Latitude:  -6.2088,
		Longitude: 106.8456,
		Timestamp: 1715003456,
	}

	tests := []struct {
		name     string
		location Location
		wantErr  bool
	}{
		{
			name:     "valid location",
			location: validLocation,
		},
		{
			name: "empty vehicle ID",
			location: Location{
				Latitude:  -6.2088,
				Longitude: 106.8456,
				Timestamp: 1715003456,
			},
			wantErr: true,
		},
		{
			name: "latitude out of range",
			location: Location{
				VehicleID: "B1234XYZ",
				Latitude:  91,
				Longitude: 106.8456,
				Timestamp: 1715003456,
			},
			wantErr: true,
		},
		{
			name: "longitude is not finite",
			location: Location{
				VehicleID: "B1234XYZ",
				Latitude:  -6.2088,
				Longitude: math.NaN(),
				Timestamp: 1715003456,
			},
			wantErr: true,
		},
		{
			name: "zero timestamp",
			location: Location{
				VehicleID: "B1234XYZ",
				Latitude:  -6.2088,
				Longitude: 106.8456,
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.location.Validate()

			if test.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("Validate returned error: %v", err)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{
			name: "valid payload",
			payload: `{
				"vehicle_id":"B1234XYZ",
				"latitude":-6.2088,
				"longitude":106.8456,
				"timestamp":1715003456
			}`,
		},
		{
			name: "malformed JSON",
			payload: `{
				"vehicle_id":"B1234XYZ"
			`,
			wantErr: true,
		},
		{
			name: "unknown field",
			payload: `{
				"vehicle_id":"B1234XYZ",
				"latitude":-6.2088,
				"longitude":106.8456,
				"timestamp":1715003456,
				"unexpected":true
			}`,
			wantErr: true,
		},
		{
			name: "two JSON objects",
			payload: `{"vehicle_id":"B1234XYZ","latitude":-6.2088,"longitude":106.8456,"timestamp":1715003456}
{"vehicle_id":"B5678ABC","latitude":-6.2,"longitude":106.8,"timestamp":1715003457}`,
			wantErr: true,
		},
		{
			name:    "null payload",
			payload: `null`,
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			location, err := Decode([]byte(test.payload))

			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("Decode returned error: %v", err)
			}
			if location.VehicleID != "B1234XYZ" {
				t.Fatalf("got vehicle ID %q", location.VehicleID)
			}
		})
	}
}

func TestValidateVehicleID(t *testing.T) {
	tests := []struct {
		name      string
		vehicleID string
		wantErr   bool
	}{
		{name: "letters numbers hyphen and underscore", vehicleID: "B1234-XYZ_test"},
		{name: "empty", vehicleID: "", wantErr: true},
		{name: "whitespace only", vehicleID: "   ", wantErr: true},
		{name: "space is invalid", vehicleID: "B 1234XYZ", wantErr: true},
		{name: "too long", vehicleID: strings.Repeat("A", 65), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateVehicleID(test.vehicleID)

			if test.wantErr && err == nil {
				t.Fatal("expected error")
			}

			if !test.wantErr && err != nil {
				t.Fatalf("ValidateVehicleID returned error: %v", err)
			}
		})
	}
}
