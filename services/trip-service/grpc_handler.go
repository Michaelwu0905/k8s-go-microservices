package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Trip 是内存中的订单实体
type Trip struct {
	ID       string
	Status   pb.TripStatus
	DriverID string
	Route    *pb.Route
}

// TripStore 是线程安全的订单内存存储（demo 用，生产应换成数据库）
type TripStore struct {
	mu    sync.RWMutex
	trips map[string]*Trip
}

func NewTripStore() *TripStore {
	return &TripStore{trips: make(map[string]*Trip)}
}

func (s *TripStore) Create(t *Trip) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trips[t.ID] = t
}

func (s *TripStore) Get(id string) (*Trip, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.trips[id]
	return t, ok
}

func (s *TripStore) Update(id string, update func(*Trip)) (*Trip, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.trips[id]
	if !ok {
		return nil, false
	}
	update(t)
	return t, true
}

type TripGrpcHandler struct {
	pb.UnimplementedTripServiceServer

	store *TripStore
}

func NewGrpcHandler(s *grpc.Server) {
	handler := &TripGrpcHandler{
		store: NewTripStore(),
	}

	pb.RegisterTripServiceServer(s, handler)
}

func generateTripID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// fetchRoute 调用 OSRM 公共路由服务计算驾车路线
func fetchRoute(pickup, destination *pb.Coordinate) (*pb.Route, error) {
	url := fmt.Sprintf(
		"https://router.project-osrm.org/route/v1/driving/%f,%f;%f,%f?overview=full&geometries=geojson",
		pickup.Longitude, pickup.Latitude,
		destination.Longitude, destination.Latitude,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch route: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read response: %v", err)
	}

	type OSRMResponse struct {
		Routes []struct {
			Distance float64 `json:"distance"`
			Duration float64 `json:"duration"`
			Geometry struct {
				Coordinates [][]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"routes"`
	}
	var routeResp OSRMResponse

	if err := json.Unmarshal(body, &routeResp); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse response: %v", err)
	}

	if len(routeResp.Routes) == 0 {
		return nil, status.Errorf(codes.Internal, "no route found")
	}

	route := routeResp.Routes[0]
	geometry := route.Geometry.Coordinates
	coordinates := make([]*pb.Coordinate, len(geometry))
	for i, coord := range geometry {
		// OSRM 返回 GeoJSON [longitude, latitude]
		coordinates[i] = &pb.Coordinate{
			Latitude:  float32(coord[1]),
			Longitude: float32(coord[0]),
		}
	}

	return &pb.Route{
		Geometry: []*pb.Geometry{
			{
				Coordinates: coordinates,
			},
		},
		Distance: float32(route.Distance),
		Duration: float32(route.Duration),
	}, nil
}

// CreateTrip 计算路线并创建订单（初始状态 PENDING）
func (h *TripGrpcHandler) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {
	route, err := fetchRoute(req.StartLocation, req.EndLocation)
	if err != nil {
		return nil, err
	}

	trip := &Trip{
		ID:     generateTripID(),
		Status: pb.TripStatus_TRIP_STATUS_PENDING,
		Route:  route,
	}
	h.store.Create(trip)

	return &pb.CreateTripResponse{
		TripId: trip.ID,
		Route:  route,
		Status: trip.Status,
	}, nil
}

// AcceptTrip 司机接单：状态 PENDING -> ACCEPTED
func (h *TripGrpcHandler) AcceptTrip(ctx context.Context, req *pb.AcceptTripRequest) (*pb.AcceptTripResponse, error) {
	trip, ok := h.store.Update(req.TripId, func(t *Trip) {
		t.Status = pb.TripStatus_TRIP_STATUS_ACCEPTED
		t.DriverID = req.DriverId
	})
	if !ok {
		return nil, status.Errorf(codes.NotFound, "trip not found: %s", req.TripId)
	}

	return &pb.AcceptTripResponse{
		TripId: trip.ID,
		Status: trip.Status,
	}, nil
}

// GetTrip 查询订单状态
func (h *TripGrpcHandler) GetTrip(ctx context.Context, req *pb.GetTripRequest) (*pb.GetTripResponse, error) {
	trip, ok := h.store.Get(req.TripId)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "trip not found: %s", req.TripId)
	}

	return &pb.GetTripResponse{
		TripId: trip.ID,
		Status: trip.Status,
	}, nil
}
