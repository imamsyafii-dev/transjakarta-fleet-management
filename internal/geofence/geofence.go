package geofence

import (
	"fmt"
	"math"
	"sync"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

const earthRadiusMeters = 6_371_000

type Config struct {
	CenterLatitude  float64
	CenterLongitude float64
	RadiusMeters    float64
}

type Observation struct {
	DistanceMeters float64
	Inside         bool
	// Entered is true only after a previously observed outside location becomes inside.
	// A vehicle first observed inside does not emit an entry event.
	Entered bool
}

type Detector struct {
	centerLatitude  float64
	centerLongitude float64
	radiusMeters    float64

	mu     sync.Mutex
	inside map[string]bool
}

func New(config Config) (*Detector, error) {
	if err := validateCoordinates(
		config.CenterLatitude,
		config.CenterLongitude,
	); err != nil {
		return nil, fmt.Errorf("validate geofence center: %w", err)
	}
	if math.IsNaN(config.RadiusMeters) || math.IsInf(config.RadiusMeters, 0) {
		return nil, fmt.Errorf("geofence radius must be a finite number")
	}
	if config.RadiusMeters <= 0 {
		return nil, fmt.Errorf("geofence radius must be greater than zero")
	}

	return &Detector{
		centerLatitude:  config.CenterLatitude,
		centerLongitude: config.CenterLongitude,
		radiusMeters:    config.RadiusMeters,
		inside:          make(map[string]bool),
	}, nil
}

func (detector *Detector) Observe(value location.Location) (Observation, error) {
	if detector == nil {
		return Observation{}, fmt.Errorf("geofence detector is not initialized")
	}
	if err := value.Validate(); err != nil {
		return Observation{}, fmt.Errorf("validate location: %w", err)
	}

	distanceMeters := DistanceMeters(
		value.Latitude,
		value.Longitude,
		detector.centerLatitude,
		detector.centerLongitude,
	)
	inside := distanceMeters <= detector.radiusMeters

	detector.mu.Lock()
	wasInside, known := detector.inside[value.VehicleID]
	detector.inside[value.VehicleID] = inside
	detector.mu.Unlock()

	return Observation{
		DistanceMeters: distanceMeters,
		Inside:         inside,
		Entered:        known && !wasInside && inside,
	}, nil
}

func DistanceMeters(
	latitudeA float64,
	longitudeA float64,
	latitudeB float64,
	longitudeB float64,
) float64 {
	latitudeDelta := radians(latitudeB - latitudeA)
	longitudeDelta := radians(longitudeB - longitudeA)

	a := math.Sin(latitudeDelta/2)*math.Sin(latitudeDelta/2) +
		math.Cos(radians(latitudeA))*math.Cos(radians(latitudeB))*
			math.Sin(longitudeDelta/2)*math.Sin(longitudeDelta/2)
	a = math.Max(0, math.Min(1, a))

	return earthRadiusMeters * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func validateCoordinates(latitude float64, longitude float64) error {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) {
		return fmt.Errorf("latitude must be a finite number")
	}
	if latitude < -90 || latitude > 90 {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if math.IsNaN(longitude) || math.IsInf(longitude, 0) {
		return fmt.Errorf("longitude must be a finite number")
	}
	if longitude < -180 || longitude > 180 {
		return fmt.Errorf("longitude must be between -180 and 180")

	}

	return nil
}

func radians(value float64) float64 {
	return value * math.Pi / 180
}
