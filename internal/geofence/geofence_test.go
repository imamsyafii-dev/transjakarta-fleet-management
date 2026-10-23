package geofence

import (
	"math"
	"testing"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "latitude out of range",
			config: Config{
				CenterLatitude: 91,
				RadiusMeters:   50,
			},
		},
		{
			name: "longitude is not finite",
			config: Config{
				CenterLongitude: math.NaN(),
				RadiusMeters:    50,
			},
		},
		{
			name:   "zero radius",
			config: Config{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detector, err := New(test.config)
			if err == nil {
				t.Fatal("expected error")
			}
			if detector != nil {
				t.Fatal("expected nil detector")
			}
		})
	}
}

func TestDistanceMeters(t *testing.T) {
	if distance := DistanceMeters(0, 0, 0, 0); distance != 0 {
		t.Fatalf("distance = %f, want 0", distance)
	}

	distance := DistanceMeters(0, 0, 0, 1)
	if math.Abs(distance-111_195) > 100 {
		t.Fatalf("distance = %f, want approximately 111195", distance)
	}
}

func TestObserveEmitsOnlyOutsideToInsideTransition(t *testing.T) {
	detector, err := New(Config{
		CenterLatitude:  0,
		CenterLongitude: 0,
		RadiusMeters:    20,
	})
	if err != nil {
		t.Fatalf("create detector: %v", err)
	}

	outside := sampleLocation("B1234XYZ", 0.0010)
	inside := sampleLocation("B1234XYZ", 0.0001)

	observations := []struct {
		location    location.Location
		wantInside  bool
		wantEntered bool
	}{
		{location: outside, wantInside: false, wantEntered: false},
		{location: inside, wantInside: true, wantEntered: true},
		{location: inside, wantInside: true, wantEntered: false},
		{location: outside, wantInside: false, wantEntered: false},
		{location: inside, wantInside: true, wantEntered: true},
	}

	for index, test := range observations {
		observation, err := detector.Observe(test.location)
		if err != nil {
			t.Fatalf("observe at index %d: %v", index, err)
		}
		if observation.Inside != test.wantInside {
			t.Fatalf(
				"inside at index %d = %t, want %t",
				index,
				observation.Inside,
				test.wantInside,
			)
		}
		if observation.Entered != test.wantEntered {
			t.Fatalf(
				"entered at index %d = %t, want %t",
				index,
				observation.Entered,
				test.wantEntered,
			)
		}
	}
}

func TestObserveTracksVehiclesIndependently(t *testing.T) {
	detector, err := New(Config{RadiusMeters: 20})
	if err != nil {
		t.Fatalf("create detector: %v", err)
	}

	if _, err := detector.Observe(sampleLocation("B1234XYZ", 0.0010)); err != nil {
		t.Fatalf("observe first vehicle outside: %v", err)
	}
	if _, err := detector.Observe(sampleLocation("B5678ABC", 0.0010)); err != nil {
		t.Fatalf("observe second vehicle outside: %v", err)
	}

	first, err := detector.Observe(sampleLocation("B1234XYZ", 0.0001))
	if err != nil {
		t.Fatalf("observe first vehicle inside: %v", err)
	}
	second, err := detector.Observe(sampleLocation("B5678ABC", 0.0001))
	if err != nil {
		t.Fatalf("observe second vehicle inside: %v", err)
	}

	if !first.Entered || !second.Entered {
		t.Fatalf("expected each vehicle to emit its own entry event")
	}
}

func sampleLocation(vehicleID string, latitude float64) location.Location {
	return location.Location{
		VehicleID: vehicleID,
		Latitude:  latitude,
		Longitude: 0,
		Timestamp: 1,
	}
}
