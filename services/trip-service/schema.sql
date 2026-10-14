CREATE TABLE IF NOT EXISTS trips (
    id TEXT PRIMARY KEY,
    rider_id TEXT NOT NULL,
    driver_id TEXT NOT NULL DEFAULT '',
    status SMALLINT NOT NULL CHECK (status BETWEEN 1 AND 6),
    pickup_lat DOUBLE PRECISION NOT NULL,
    pickup_lng DOUBLE PRECISION NOT NULL,
    destination_lat DOUBLE PRECISION NOT NULL,
    destination_lng DOUBLE PRECISION NOT NULL,
    route JSONB NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status NOT IN (2, 3, 4, 5) OR driver_id <> '')
);
CREATE UNIQUE INDEX IF NOT EXISTS one_active_trip_per_driver
ON trips (driver_id) WHERE driver_id <> '' AND status IN (2, 3, 4);
CREATE UNIQUE INDEX IF NOT EXISTS one_active_trip_per_rider
ON trips (rider_id) WHERE status IN (1, 2, 3, 4);
CREATE INDEX IF NOT EXISTS trips_pending ON trips (created_at) WHERE status = 1;
CREATE INDEX IF NOT EXISTS trips_rider ON trips (rider_id, created_at DESC);
CREATE INDEX IF NOT EXISTS trips_driver ON trips (driver_id, created_at DESC);
