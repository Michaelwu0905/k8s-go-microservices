package main

import (
	"context"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"math"
	"net/http"
	"net/http/httptest"
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
