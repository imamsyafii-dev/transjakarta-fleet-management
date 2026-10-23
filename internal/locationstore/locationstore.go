package locationstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/imamsyafii-dev/transjakarta-fleet-management/internal/location"
)

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

func (store *Store) Close() {
	if store != nil && store.pool != nil {
		store.pool.Close()
	}
}
