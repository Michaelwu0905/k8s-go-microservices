package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func integrationStores(t *testing.T) (*TripStore, *TripStore) {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("trip_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		admin.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{name}.Sanitize()+" CASCADE")
		admin.Close(ctx)
	})
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", name)
	parsed.RawQuery = query.Encode()
	stores := make([]*TripStore, 2)
	for i := range stores {
		stores[i], err = NewTripStore(ctx, parsed.String())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(stores[i].pool.Close)
	}
	return stores[0], stores[1]
}

func seedTrip(t *testing.T, s *TripStore, id, rider string) *pb.Trip {
	t.Helper()
	trip, err := s.Create(context.Background(), &pb.Trip{TripId: id, RiderId: rider,
		Pickup: &pb.Coordinate{Latitude: 39.9, Longitude: 116.4}, Destination: &pb.Coordinate{Latitude: 39.91, Longitude: 116.41},
		Route: &pb.Route{Distance: 2000, Duration: 300, Geometry: []*pb.Geometry{{Coordinates: []*pb.Coordinate{{Latitude: 39.9, Longitude: 116.4}, {Latitude: 39.91, Longitude: 116.41}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	return trip
}

func TestPostgresPersistenceContentionAndNotifications(t *testing.T) {
	a, b := integrationStores(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	NewGrpcHandler(server, a)
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///trips", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	client := pb.NewTripServiceClient(conn)
	inbox, err := client.WatchTrips(ctx, &pb.WatchTripsRequest{DriverId: "driver-0"})
	if err != nil {
		t.Fatal(err)
	}
	initial, err := inbox.Recv()
	if err != nil || len(initial.Trips) != 0 {
		t.Fatalf("initial snapshot: %v %v", initial, err)
	}
	seedTrip(t, b, "trip-first", "rider-first")
	event, err := inbox.Recv()
	if err != nil || len(event.Trips) != 1 || event.Trips[0].Status != 1 {
		t.Fatalf("cross-instance notification: %v %v", event, err)
	}
	watch, err := client.WatchTrips(ctx, &pb.WatchTripsRequest{TripId: "trip-first"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = watch.Recv(); err != nil {
		t.Fatal(err)
	}
	seedTrip(t, b, "trip-second", "rider-second")
	if _, err := b.Create(ctx, &pb.Trip{TripId: "duplicate-rider", RiderId: "rider-first", Pickup: &pb.Coordinate{}, Destination: &pb.Coordinate{}, Route: &pb.Route{}}); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate active rider: %v", err)
	}
	var wg sync.WaitGroup
	winners := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			driver := fmt.Sprintf("driver-%d", i)
			_, err := b.ChangeStatus(ctx, "trip-first", 2, driver, "", true)
			if err == nil {
				winners <- driver
			} else if status.Code(err) != codes.FailedPrecondition {
				t.Errorf("accept race: %v", err)
			}
		}(i)
	}
	wg.Wait()
	close(winners)
	var winner string
	count := 0
	for driver := range winners {
		winner = driver
		count++
	}
	if count != 1 {
		t.Fatalf("expected one winning driver, got %d", count)
	}
	if _, err = b.ChangeStatus(ctx, "trip-second", 2, winner, "", true); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("driver accepted two active trips: %v", err)
	}
	if _, err = b.ChangeStatus(ctx, "trip-first", 3, "wrong-driver", "", false); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong driver: %v", err)
	}
	if _, err = b.ChangeStatus(ctx, "trip-first", 5, winner, "", false); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("skipped state: %v", err)
	}
	for _, next := range []pb.TripStatus{3, 4, 5} {
		updated, err := b.ChangeStatus(ctx, "trip-first", next, winner, "", false)
		if err != nil {
			t.Fatal(err)
		}
		for {
			snapshot, err := watch.Recv()
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.Trips[0].Version >= updated.Version {
				if snapshot.Trips[0].Status != next {
					t.Fatalf("unexpected event %v", snapshot)
				}
				break
			}
		}
	}
	persisted, err := a.Get(ctx, "trip-first")
	if err != nil || persisted.Status != 5 || persisted.DriverId != winner || persisted.Version != 5 || persisted.Route.Distance != 2000 {
		t.Fatalf("persisted state: %v %v", persisted, err)
	}
	// Reopen the repository to model a service restart; data must survive.
	reopened, err := NewTripStore(ctx, a.databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := reopened.Get(ctx, "trip-first")
	reopened.pool.Close()
	if err != nil || restored.Status != 5 {
		t.Fatalf("restart recovery: %v %v", restored, err)
	}
	if _, err = b.ChangeStatus(ctx, "trip-second", 2, winner, "", true); err != nil {
		t.Fatalf("completed trip did not release driver: %v", err)
	}
	if _, err = b.ChangeStatus(ctx, "trip-second", 6, "", "rider-second", false); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresCancelAcceptRace(t *testing.T) {
	a, b := integrationStores(t)
	seedTrip(t, a, "race-trip", "race-rider")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := a.ChangeStatus(context.Background(), "race-trip", 2, "driver-0", "", true)
		if err != nil && status.Code(err) != codes.FailedPrecondition {
			t.Error(err)
		}
	}()
	go func() {
		defer wg.Done()
		_, err := b.ChangeStatus(context.Background(), "race-trip", 6, "", "race-rider", false)
		if err != nil {
			t.Error(err)
		}
	}()
	wg.Wait()
	result, err := a.Get(context.Background(), "race-trip")
	if err != nil || result.Status != 6 {
		t.Fatalf("cancel must win eventually: %v %v", result, err)
	}
}
