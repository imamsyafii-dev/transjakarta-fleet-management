package locationstore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

var ErrNotFound = errors.New("vehicle location not found")

type Saver interface {
	Save(context.Context, location.Location) error
}

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database URL must not be empty")
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (store *Store) Save(ctx context.Context, value location.Location) error {
	if store == nil || store.pool == nil {
		return fmt.Errorf("location store is not initialized")
	}

	if err := value.Validate(); err != nil {
		return fmt.Errorf("validate location before save: %w", err)
	}

	_, err := store.pool.Exec(
		ctx,
		`INSERT INTO vehicle_locations (vehicle_id, latitude, longitude, timestamp)
		 VALUES ($1, $2, $3, $4)`,
		value.VehicleID,
		value.Latitude,
		value.Longitude,
		value.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert vehicle location: %w", err)
	}

	return nil
}

func (store *Store) Latest(
	ctx context.Context,
	vehicleID string,
) (location.Location, error) {
	if err := location.ValidateVehicleID(vehicleID); err != nil {
		return location.Location{}, fmt.Errorf("validate vehicle ID: %w", err)
	}

	if store == nil || store.pool == nil {
		return location.Location{}, fmt.Errorf("location store is not initialized")
	}

	var value location.Location

	err := store.pool.QueryRow(
		ctx,
		`SELECT vehicle_id, latitude, longitude, timestamp
		 FROM vehicle_locations
		 WHERE vehicle_id = $1
		 ORDER BY timestamp DESC, id DESC
		 LIMIT 1`,
		vehicleID,
	).Scan(
		&value.VehicleID,
		&value.Latitude,
		&value.Longitude,
		&value.Timestamp,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return location.Location{}, fmt.Errorf(
			"%w: vehicle_id=%q",
			ErrNotFound,
			vehicleID,
		)
	}
	if err != nil {
		return location.Location{}, fmt.Errorf(
			"query latest vehicle location: %w",
			err,
		)
	}

	return value, nil
}

func (store *Store) History(
	ctx context.Context,
	vehicleID string,
	start int64,
	end int64,
) ([]location.Location, error) {
	if err := location.ValidateVehicleID(vehicleID); err != nil {
		return nil, fmt.Errorf("validate vehicle ID: %w", err)
	}
	if start <= 0 {
		return nil, fmt.Errorf("history start must be greater than zero")
	}
	if end <= 0 {
		return nil, fmt.Errorf("history end must be greater than zero")
	}
	if start > end {
		return nil, fmt.Errorf("history start must not be after end")
	}
	if store == nil || store.pool == nil {
		return nil, fmt.Errorf("location store is not initialized")
	}

	rows, err := store.pool.Query(
		ctx,
		`SELECT vehicle_id, latitude, longitude, timestamp
		 FROM vehicle_locations
		 WHERE vehicle_id = $1 AND timestamp BETWEEN $2 AND $3
		 ORDER BY timestamp ASC, id ASC`,
		vehicleID,
		start,
		end,
	)
	if err != nil {
		return nil, fmt.Errorf("query vehicle location history: %w", err)
	}
	defer rows.Close()

	values := make([]location.Location, 0)

	for rows.Next() {
		var value location.Location

		if err := rows.Scan(
			&value.VehicleID,
			&value.Latitude,
			&value.Longitude,
			&value.Timestamp,
		); err != nil {
			return nil, fmt.Errorf(
				"scan vehicle location history: %w",
				err,
			)
		}

		values = append(values, value)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf(
			"iterate vehicle location history: %w",
			err,
		)
	}

	return values, nil
}

func (store *Store) Close() {
	if store != nil && store.pool != nil {
		store.pool.Close()
	}
}
