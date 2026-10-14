package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

//go:embed schema.sql
var schema string

type TripStore struct {
	pool        *pgxpool.Pool
	databaseURL string
}

func NewTripStore(ctx context.Context, databaseURL string) (*TripStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	s := &TripStore{pool: pool, databaseURL: databaseURL}
	tx, err := pool.Begin(ctx)
	if err != nil {
		pool.Close()
		return nil, err
	}
	defer tx.Rollback(ctx)
	// Serialize migrations when several replicas start together.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(734893201)"); err == nil {
		_, err = tx.Exec(ctx, schema)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		_ = tx.Rollback(ctx)
		pool.Close()
		return nil, err
	}
	return s, nil
}

const tripColumns = `id, status, driver_id, rider_id, pickup_lat, pickup_lng,
destination_lat, destination_lng, route, created_at, updated_at, version`

func scanTrip(row pgx.Row) (*pb.Trip, error) {
	t := &pb.Trip{Pickup: &pb.Coordinate{}, Destination: &pb.Coordinate{}, Route: &pb.Route{}}
	var route []byte
	var created, updated time.Time
	err := row.Scan(&t.TripId, &t.Status, &t.DriverId, &t.RiderId,
		&t.Pickup.Latitude, &t.Pickup.Longitude, &t.Destination.Latitude, &t.Destination.Longitude,
		&route, &created, &updated, &t.Version)
	if err != nil {
		return nil, databaseError(err)
	}
	if err = json.Unmarshal(route, t.Route); err != nil {
		return nil, status.Error(codes.Internal, "invalid stored route")
	}
	t.CreatedAt = created.UTC().Format(time.RFC3339Nano)
	t.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return t, nil
}

func databaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return status.Error(codes.NotFound, "trip not found")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return status.Error(codes.AlreadyExists, "driver or rider already has an active trip")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "request cancelled")
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "database request timed out")
	}
	return status.Error(codes.Unavailable, "trip database unavailable")
}

func (s *TripStore) Create(ctx context.Context, t *pb.Trip) (*pb.Trip, error) {
	route, err := json.Marshal(t.Route)
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot encode route")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	defer tx.Rollback(ctx)
	result, err := scanTrip(tx.QueryRow(ctx, `INSERT INTO trips
		(id, rider_id, status, pickup_lat, pickup_lng, destination_lat, destination_lng, route)
		VALUES ($1,$2,1,$3,$4,$5,$6,$7) RETURNING `+tripColumns,
		t.TripId, t.RiderId, t.Pickup.Latitude, t.Pickup.Longitude, t.Destination.Latitude, t.Destination.Longitude, route))
	if err != nil {
		return nil, err
	}
	if err = notifyAndCommit(ctx, tx, t.TripId); err != nil {
		return nil, err
	}
	return result, nil
}

func notifyAndCommit(ctx context.Context, tx pgx.Tx, id string) error {
	// NOTIFY is delivered only after the transaction commits successfully.
	if _, err := tx.Exec(ctx, "SELECT pg_notify('trip_events', $1)", id); err != nil {
		return databaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return databaseError(err)
	}
	return nil
}

func (s *TripStore) Get(ctx context.Context, id string) (*pb.Trip, error) {
	return scanTrip(s.pool.QueryRow(ctx, "SELECT "+tripColumns+" FROM trips WHERE id=$1", id))
}

func (s *TripStore) List(ctx context.Context, driverID, riderID string) ([]*pb.Trip, error) {
	condition, identity := "rider_id=$1", riderID
	ordering := "(status IN (1,2,3,4)) DESC, created_at DESC"
	if driverID != "" {
		condition, identity = "(driver_id=$1 OR status=1)", driverID
		ordering = "(driver_id=$1 AND status IN (2,3,4)) DESC, (status=1) DESC, created_at DESC"
	}
	rows, err := s.pool.Query(ctx, "SELECT "+tripColumns+" FROM trips WHERE "+condition+
		" ORDER BY "+ordering+" LIMIT 100", identity)
	if err != nil {
		return nil, databaseError(err)
	}
	defer rows.Close()
	trips := make([]*pb.Trip, 0)
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			return nil, err
		}
		trips = append(trips, t)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseError(err)
	}
	return trips, nil
}

func (s *TripStore) ChangeStatus(ctx context.Context, id string, next pb.TripStatus, driverID, riderID string, accepting bool) (*pb.Trip, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	defer tx.Rollback(ctx)
	t, err := scanTrip(tx.QueryRow(ctx, "SELECT "+tripColumns+" FROM trips WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return nil, err
	}
	if accepting {
		if t.Status == pb.TripStatus_TRIP_STATUS_ACCEPTED && t.DriverId == driverID {
			return t, nil
		}
		if t.Status != pb.TripStatus_TRIP_STATUS_PENDING {
			return nil, status.Error(codes.FailedPrecondition, "trip is no longer available")
		}
		t.DriverId = driverID
	} else if err := checkTransition(t, next, driverID, riderID); err != nil {
		return nil, err
	}
	if t.Status == next {
		return t, nil
	}
	result, err := scanTrip(tx.QueryRow(ctx, `UPDATE trips SET status=$2, driver_id=$3,
		version=version+1, updated_at=clock_timestamp() WHERE id=$1 RETURNING `+tripColumns, id, next, t.DriverId))
	if err != nil {
		return nil, err
	}
	if err = notifyAndCommit(ctx, tx, id); err != nil {
		return nil, err
	}
	return result, nil
}
