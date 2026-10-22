CREATE TABLE vehicle_locations (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  vehicle_id TEXT NOT NULL CHECK (btrim(vehicle_id) <> ''),
  latitude DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
  longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
  timestamp BIGINT NOT NULL CHECK (timestamp > 0)
);

CREATE INDEX idx_vehicle_locations_vehicle_timestamp
  ON vehicle_locations (vehicle_id, timestamp DESC, id DESC);
