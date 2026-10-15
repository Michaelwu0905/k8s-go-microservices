package main

import (
	"context"
	"math"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mmcloughlin/geohash"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/driver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const locationSchema = `CREATE TABLE IF NOT EXISTS driver_locations (
 driver_id TEXT PRIMARY KEY,
 online BOOLEAN NOT NULL DEFAULT false,
 has_location BOOLEAN NOT NULL DEFAULT false,
 latitude DOUBLE PRECISION NOT NULL DEFAULT 0,
 longitude DOUBLE PRECISION NOT NULL DEFAULT 0,
 source TEXT NOT NULL DEFAULT '',
 accuracy REAL NOT NULL DEFAULT 0,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`
const driverColumns = `driver_id, has_location, latitude, longitude,
(online AND has_location AND updated_at > clock_timestamp()-interval '30 seconds'), source, accuracy, updated_at`

var driverIdentity = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type Service struct{ pool *pgxpool.Pool }

func NewService(ctx context.Context, databaseURL string) (*Service, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(734893202)"); err == nil {
		_, err = tx.Exec(ctx, locationSchema)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		pool.Close()
		return nil, err
	}
	return &Service{pool: pool}, nil
}

func validLocation(location *pb.Location) bool {
	if location == nil {
		return false
	}
	lat, lng := float64(location.Latitude), float64(location.Longitude)
	return !math.IsNaN(lat) && !math.IsNaN(lng) && lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

func scanDriver(row pgx.Row) (*pb.Driver, error) {
	driver := &pb.Driver{}
	var located bool
	var lat, lng float64
	var updated time.Time
	if err := row.Scan(&driver.DriverId, &located, &lat, &lng, &driver.Online, &driver.Source, &driver.Accuracy, &updated); err != nil {
		return nil, err
	}
	if located {
		driver.Location = &pb.Location{Latitude: float32(lat), Longitude: float32(lng)}
		driver.Geohash = geohash.EncodeWithPrecision(lat, lng, 6)
		driver.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	}
	return driver, nil
}
func driverDBError(err error) error {
	if err == pgx.ErrNoRows {
		return status.Error(codes.NotFound, "driver not found")
	}
	return status.Error(codes.Unavailable, "driver location database unavailable")
}
func (s *Service) Get(ctx context.Context, id string) (*pb.Driver, error) {
	d, err := scanDriver(s.pool.QueryRow(ctx, "SELECT "+driverColumns+" FROM driver_locations WHERE driver_id=$1", id))
	if err == pgx.ErrNoRows {
		return &pb.Driver{DriverId: id}, nil
	}
	if err != nil {
		return nil, driverDBError(err)
	}
	return d, nil
}
func (s *Service) SetAvailability(ctx context.Context, id string, online bool) (*pb.Driver, error) {
	d, err := scanDriver(s.pool.QueryRow(ctx, `INSERT INTO driver_locations(driver_id,online) VALUES($1,$2)
 ON CONFLICT(driver_id) DO UPDATE SET online=EXCLUDED.online RETURNING `+driverColumns, id, online))
	if err != nil {
		return nil, driverDBError(err)
	}
	return d, nil
}
func (s *Service) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.Driver, error) {
	d, err := scanDriver(s.pool.QueryRow(ctx, `INSERT INTO driver_locations
 (driver_id,has_location,latitude,longitude,source,accuracy,updated_at) VALUES($1,true,$2,$3,$4,$5,clock_timestamp())
 ON CONFLICT(driver_id) DO UPDATE SET has_location=true,latitude=EXCLUDED.latitude,longitude=EXCLUDED.longitude,
 source=EXCLUDED.source,accuracy=EXCLUDED.accuracy,updated_at=EXCLUDED.updated_at RETURNING `+driverColumns,
		req.DriverId, req.Location.Latitude, req.Location.Longitude, req.Source, req.Accuracy))
	if err != nil {
		return nil, driverDBError(err)
	}
	return d, nil
}

func distanceMeters(a, b *pb.Location) float64 {
	const rad = math.Pi / 180
	lat := (float64(b.Latitude) - float64(a.Latitude)) * rad
	lng := (float64(b.Longitude) - float64(a.Longitude)) * rad
	v := math.Sin(lat/2)*math.Sin(lat/2) + math.Cos(float64(a.Latitude)*rad)*math.Cos(float64(b.Latitude)*rad)*math.Sin(lng/2)*math.Sin(lng/2)
	return 6371000 * 2 * math.Asin(math.Sqrt(math.Min(1, v)))
}
func (s *Service) FindNearbyDrivers(ctx context.Context, location *pb.Location, assigned string) ([]*pb.Driver, error) {
	rows, err := s.pool.Query(ctx, "SELECT "+driverColumns+` FROM driver_locations
 WHERE has_location AND ((online AND updated_at>clock_timestamp()-interval '30 seconds') OR driver_id=$1) ORDER BY driver_id`, assigned)
	if err != nil {
		return nil, driverDBError(err)
	}
	defer rows.Close()
	drivers := make([]*pb.Driver, 0)
	for rows.Next() {
		d, err := scanDriver(rows)
		if err != nil {
			return nil, driverDBError(err)
		}
		d.DistanceMeters = distanceMeters(location, d.Location)
		if d.DistanceMeters <= 5000 || d.DriverId == assigned {
			drivers = append(drivers, d)
		}
	}
	if err = rows.Err(); err != nil {
		return nil, driverDBError(err)
	}
	return drivers, nil
}
