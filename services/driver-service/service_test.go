package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/driver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math"
	"net/url"
	"os"
	"testing"
	"time"
)

func TestLocationValidationAndDistance(t *testing.T) {
	for _, location := range []*pb.Location{nil, {Latitude: 91}, {Longitude: 181}, {Latitude: float32(math.NaN())}, {Longitude: float32(math.Inf(1))}} {
		if validLocation(location) {
			t.Errorf("accepted invalid location: %v", location)
		}
	}
	a := &pb.Location{Latitude: 39.9, Longitude: 116.4}
	b := &pb.Location{Latitude: 39.91, Longitude: 116.4}
	if distance := distanceMeters(a, b); distance < 1100 || distance > 1130 {
		t.Fatalf("bad distance %f", distance)
	}
	handler := &DriverGrpcHandler{}
	for _, req := range []*pb.UpdateLocationRequest{nil, {DriverId: "a", Location: a, Source: "fake"}, {DriverId: "a", Location: a, Source: "gps", Accuracy: -1}} {
		if _, err := handler.UpdateLocation(context.Background(), req); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("invalid report: %v", err)
		}
	}
}

func TestPostgresDriverLocations(t *testing.T) {
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set TEST_DATABASE_URL for location integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("driver_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{name}.Sanitize()+" CASCADE")
		admin.Close(context.Background())
	}()
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	params := parsed.Query()
	params.Set("search_path", name)
	parsed.RawQuery = params.Encode()
	a, err := NewService(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer a.pool.Close()
	b, err := NewService(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer b.pool.Close()
	if _, err = a.SetAvailability(ctx, "driver-0", true); err != nil {
		t.Fatal(err)
	}
	list, err := b.FindNearbyDrivers(ctx, &pb.Location{Latitude: 39.9, Longitude: 116.4}, "")
	if err != nil || len(list) != 0 {
		t.Fatalf("driver without coordinates should be hidden: %v %v", list, err)
	}
	report := &pb.UpdateLocationRequest{DriverId: "driver-0", Location: &pb.Location{Latitude: 39.901, Longitude: 116.4}, Source: "gps", Accuracy: 8}
	if _, err = a.UpdateLocation(ctx, report); err != nil {
		t.Fatal(err)
	}
	list, err = b.FindNearbyDrivers(ctx, &pb.Location{Latitude: 39.9, Longitude: 116.4}, "")
	if err != nil || len(list) != 1 || !list[0].Online || list[0].Source != "gps" || list[0].DistanceMeters > 120 {
		t.Fatalf("cross-instance report: %v %v", list, err)
	}
	if _, err = a.pool.Exec(ctx, "UPDATE driver_locations SET updated_at=now()-interval '31 seconds'"); err != nil {
		t.Fatal(err)
	}
	list, err = b.FindNearbyDrivers(ctx, report.Location, "")
	if err != nil || len(list) != 0 {
		t.Fatalf("expired driver visible: %v %v", list, err)
	}
	list, err = b.FindNearbyDrivers(ctx, report.Location, "driver-0")
	if err != nil || len(list) != 1 || list[0].Online {
		t.Fatalf("assigned expired driver should remain marked offline: %v %v", list, err)
	}
	if _, err = a.SetAvailability(ctx, "driver-0", false); err != nil {
		t.Fatal(err)
	}
	if _, err = a.UpdateLocation(ctx, report); err != nil {
		t.Fatal(err)
	}
	restored, err := b.Get(ctx, "driver-0")
	if err != nil || restored.Online || restored.Location == nil {
		t.Fatalf("late report revived offline driver: %v %v", restored, err)
	}
	if _, err = a.SetAvailability(ctx, "driver-0", true); err != nil {
		t.Fatal(err)
	}
	list, err = b.FindNearbyDrivers(ctx, &pb.Location{Latitude: 31.2, Longitude: 121.4}, "")
	if err != nil || len(list) != 0 {
		t.Fatalf("distant driver should be excluded: %v %v", list, err)
	}
	list, err = b.FindNearbyDrivers(ctx, &pb.Location{Latitude: 31.2, Longitude: 121.4}, "driver-0")
	if err != nil || len(list) != 1 {
		t.Fatalf("assigned driver must be visible outside nearby radius: %v %v", list, err)
	}
}
