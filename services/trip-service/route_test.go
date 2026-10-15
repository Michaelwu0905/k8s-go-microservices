package main

import (
	"context"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCoordinateValidation(t *testing.T) {
	for _, c := range []*pb.Coordinate{nil, {Latitude: 91}, {Longitude: 181}, {Latitude: float32(math.NaN())}, {Longitude: float32(math.Inf(1))}} {
		if validCoordinate(c) {
			t.Errorf("accepted invalid coordinate: %v", c)
		}
	}
	if !validCoordinate(&pb.Coordinate{}) {
		t.Fatal("zero coordinates are valid")
	}
}

func TestRouteServiceFailures(t *testing.T) {
	for _, body := range []string{`{}`, `{"routes":[{"geometry":{"coordinates":[[1],[2]]}}]}`, `{"routes":[{"geometry":{"coordinates":[[1,91],[2,3]]}}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		client := &RouteClient{baseURL: server.URL, client: server.Client()}
		if _, err := client.Fetch(context.Background(), &pb.Coordinate{}, &pb.Coordinate{}); err == nil {
			t.Errorf("accepted malformed route %s", body)
		}
		server.Close()
	}
}

func TestNavigationFollowsTripPhase(t *testing.T) {
	a, _ := integrationStores(t)
	seedTrip(t, a, "nav-trip", "nav-rider")
	ctx := context.Background()
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		w.Write([]byte(`{"routes":[{"distance":1000,"duration":100,"geometry":{"coordinates":[[116.39,39.89],[116.4,39.9]]}}]}`))
	}))
	defer server.Close()
	handler := &TripGrpcHandler{store: a, router: &RouteClient{baseURL: server.URL, client: server.Client()}}
	req := &pb.GetNavigationRequest{TripId: "nav-trip", DriverId: "driver-0", Origin: &pb.Coordinate{Latitude: 39.89, Longitude: 116.39}}
	if _, err := handler.GetNavigation(ctx, req); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("unassigned navigation: %v", err)
	}
	if _, err := a.ChangeStatus(ctx, "nav-trip", 2, "driver-0", "", true); err != nil {
		t.Fatal(err)
	}
	navigation, err := handler.GetNavigation(ctx, req)
	if err != nil || navigation.Phase != "pickup" || !strings.Contains(requestedPath, ";116.400002,39.900002") {
		t.Fatalf("pickup route: %v %s %v", navigation, requestedPath, err)
	}
	for _, next := range []pb.TripStatus{3, 4} {
		if _, err := a.ChangeStatus(ctx, "nav-trip", next, "driver-0", "", false); err != nil {
			t.Fatal(err)
		}
	}
	navigation, err = handler.GetNavigation(ctx, req)
	if err != nil || navigation.Phase != "destination" || !strings.Contains(requestedPath, ";116.410004,39.910000") {
		t.Fatalf("destination route: %v %s %v", navigation, requestedPath, err)
	}
}
