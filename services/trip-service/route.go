package main

import (
	"context"
	"encoding/json"
	"fmt"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

type RouteClient struct {
	baseURL string
	client  *http.Client
}

func NewRouteClient() *RouteClient {
	baseURL := os.Getenv("OSRM_URL")
	if baseURL == "" {
		baseURL = "https://router.project-osrm.org"
	}
	return &RouteClient{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}
func validCoordinate(c *pb.Coordinate) bool {
	if c == nil {
		return false
	}
	lat, lng := float64(c.Latitude), float64(c.Longitude)
	return !math.IsNaN(lat) && !math.IsNaN(lng) && lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}
func (r *RouteClient) Fetch(ctx context.Context, pickup, destination *pb.Coordinate) (*pb.Route, error) {
	url := fmt.Sprintf("%s/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson", r.baseURL, pickup.Longitude, pickup.Latitude, destination.Longitude, destination.Latitude)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, "invalid route service URL")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "route service unavailable; please retry")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, status.Error(codes.Unavailable, "route service returned an error")
	}
	var result struct {
		Routes []struct {
			Distance float32 `json:"distance"`
			Duration float32 `json:"duration"`
			Geometry struct {
				Coordinates [][]float32 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&result); err != nil || len(result.Routes) == 0 {
		return nil, status.Error(codes.Unavailable, "no driving route found")
	}
	route := result.Routes[0]
	if len(route.Geometry.Coordinates) < 2 || route.Distance < 0 || route.Duration < 0 {
		return nil, status.Error(codes.Unavailable, "invalid route response")
	}
	coords := make([]*pb.Coordinate, 0, len(route.Geometry.Coordinates))
	for _, coord := range route.Geometry.Coordinates {
		if len(coord) < 2 {
			return nil, status.Error(codes.Unavailable, "invalid route geometry")
		}
		point := &pb.Coordinate{Latitude: coord[1], Longitude: coord[0]}
		if !validCoordinate(point) {
			return nil, status.Error(codes.Unavailable, "invalid route coordinate")
		}
		coords = append(coords, point)
	}
	return &pb.Route{Geometry: []*pb.Geometry{{Coordinates: coords}}, Distance: route.Distance, Duration: route.Duration}, nil
}
