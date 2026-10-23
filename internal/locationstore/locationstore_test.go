package locationstore

import (
	"context"
	"testing"
)

func TestLatestRejectsInvalidVehicleID(t *testing.T) {
	store := &Store{}

	_, err := store.Latest(context.Background(), "B 1234XYZ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLatestRequiresInitializedStore(t *testing.T) {
	store := &Store{}

	_, err := store.Latest(context.Background(), "B1234XYZ")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHistoryRejectsInvalidArguments(t *testing.T) {
	store := &Store{}

	tests := []struct {
		name      string
		vehicleID string
		start     int64
		end       int64
	}{
		{name: "invalid vehicle ID", vehicleID: "B 1234XYZ", start: 1, end: 2},
		{name: "zero start", vehicleID: "B1234XYZ", start: 0, end: 2},
		{name: "zero end", vehicleID: "B1234XYZ", start: 1, end: 0},
		{name: "start after end", vehicleID: "B1234XYZ", start: 2, end: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.History(
				context.Background(),
				test.vehicleID,
				test.start,
				test.end,
			)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestHistoryRequiresInitializedStore(t *testing.T) {
	store := &Store{}

	_, err := store.History(context.Background(), "B1234XYZ", 1, 2)
	if err == nil {
		t.Fatal("expected error")
	}
}
