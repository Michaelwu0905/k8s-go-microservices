package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

type TripGrpcHandler struct {
	pb.UnimplementedTripServiceServer
	store  *TripStore
	router *RouteClient
}

func NewGrpcHandler(s *grpc.Server, store *TripStore) {
	pb.RegisterTripServiceServer(s, &TripGrpcHandler{store: store, router: NewRouteClient()})
}

func (h *TripGrpcHandler) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {
	if !validIdentity(req.GetRiderId()) || !validCoordinate(req.GetStartLocation()) || !validCoordinate(req.GetEndLocation()) {
		return nil, status.Error(codes.InvalidArgument, "valid rider_id, pickup and destination are required")
	}
	route, err := h.router.Fetch(ctx, req.StartLocation, req.EndLocation)
	if err != nil {
		return nil, err
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return nil, status.Error(codes.Internal, "cannot generate trip ID")
	}
	t, err := h.store.Create(ctx, &pb.Trip{TripId: hex.EncodeToString(id), RiderId: req.RiderId,
		Pickup: req.StartLocation, Destination: req.EndLocation, Route: route})
	if err != nil {
		return nil, err
	}
	return &pb.CreateTripResponse{TripId: t.TripId, Status: t.Status, Route: t.Route, Trip: t}, nil
}

func (h *TripGrpcHandler) AcceptTrip(ctx context.Context, req *pb.AcceptTripRequest) (*pb.AcceptTripResponse, error) {
	if !validIdentity(req.GetTripId()) || !validIdentity(req.GetDriverId()) {
		return nil, status.Error(codes.InvalidArgument, "trip_id and driver_id are required")
	}
	t, err := h.store.ChangeStatus(ctx, req.TripId, pb.TripStatus_TRIP_STATUS_ACCEPTED, req.DriverId, "", true)
	if err != nil {
		return nil, err
	}
	return &pb.AcceptTripResponse{TripId: t.TripId, Status: t.Status, Trip: t}, nil
}

func (h *TripGrpcHandler) GetTrip(ctx context.Context, req *pb.GetTripRequest) (*pb.GetTripResponse, error) {
	if !validIdentity(req.GetTripId()) {
		return nil, status.Error(codes.InvalidArgument, "trip_id is required")
	}
	t, err := h.store.Get(ctx, req.TripId)
	if err != nil {
		return nil, err
	}
	return &pb.GetTripResponse{TripId: t.TripId, Status: t.Status, Trip: t}, nil
}

func validListFilter(driverID, riderID string) bool {
	return driverID == "" && validIdentity(riderID) || riderID == "" && validIdentity(driverID)
}

func (h *TripGrpcHandler) ListTrips(ctx context.Context, req *pb.ListTripsRequest) (*pb.ListTripsResponse, error) {
	if !validListFilter(req.GetDriverId(), req.GetRiderId()) {
		return nil, status.Error(codes.InvalidArgument, "provide exactly one driver_id or rider_id")
	}
	trips, err := h.store.List(ctx, req.DriverId, req.RiderId)
	return &pb.ListTripsResponse{Trips: trips}, err
}

func (h *TripGrpcHandler) UpdateTripStatus(ctx context.Context, req *pb.UpdateTripStatusRequest) (*pb.GetTripResponse, error) {
	if !validIdentity(req.GetTripId()) || !validListFilter(req.GetDriverId(), req.GetRiderId()) {
		return nil, status.Error(codes.InvalidArgument, "trip_id and exactly one driver_id or rider_id are required")
	}
	t, err := h.store.ChangeStatus(ctx, req.TripId, req.Status, req.DriverId, req.RiderId, false)
	if err != nil {
		return nil, err
	}
	return &pb.GetTripResponse{TripId: t.TripId, Status: t.Status, Trip: t}, nil
}

func (h *TripGrpcHandler) WatchTrips(req *pb.WatchTripsRequest, stream pb.TripService_WatchTripsServer) error {
	if req.GetTripId() != "" {
		if !validIdentity(req.TripId) || req.DriverId != "" || req.RiderId != "" {
			return status.Error(codes.InvalidArgument, "provide exactly one subscription filter")
		}
	} else if !validListFilter(req.GetDriverId(), req.GetRiderId()) {
		return status.Error(codes.InvalidArgument, "provide a trip_id, driver_id or rider_id")
	}
	ctx := stream.Context()
	// Dedicated connection: waiting subscribers must not exhaust the query pool.
	connectCtx, cancelConnect := context.WithTimeout(ctx, 5*time.Second)
	conn, err := pgx.Connect(connectCtx, h.store.databaseURL)
	cancelConnect()
	if err != nil {
		return databaseError(err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = conn.Close(closeCtx)
	}()
	if _, err = conn.Exec(ctx, "LISTEN trip_events"); err != nil {
		return databaseError(err)
	}
	// Listen before reading the initial snapshot to avoid a subscription gap.
	for {
		var trips []*pb.Trip
		snapshotCtx, cancelSnapshot := context.WithTimeout(ctx, 5*time.Second)
		if req.TripId != "" {
			t, err := h.store.Get(snapshotCtx, req.TripId)
			if err != nil {
				cancelSnapshot()
				return err
			}
			trips = []*pb.Trip{t}
		} else {
			trips, err = h.store.List(snapshotCtx, req.DriverId, req.RiderId)
			if err != nil {
				cancelSnapshot()
				return err
			}
		}
		cancelSnapshot()
		if err := stream.Send(&pb.ListTripsResponse{Trips: trips}); err != nil {
			return err
		}
		waitCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		_, err = conn.WaitForNotification(waitCtx)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return databaseError(err)
		}
		// Heartbeats reconcile state even after a temporarily interrupted connection.
	}
}

func (h *TripGrpcHandler) GetNavigation(ctx context.Context, req *pb.GetNavigationRequest) (*pb.NavigationResponse, error) {
	if !validIdentity(req.GetTripId()) || !validIdentity(req.GetDriverId()) || !validCoordinate(req.GetOrigin()) {
		return nil, status.Error(codes.InvalidArgument, "trip_id, driver_id and a valid origin are required")
	}
	trip, err := h.store.Get(ctx, req.TripId)
	if err != nil {
		return nil, err
	}
	if trip.DriverId != req.DriverId {
		return nil, status.Error(codes.PermissionDenied, "only the assigned driver has a navigation route")
	}
	target, phase := trip.Pickup, "pickup"
	switch trip.Status {
	case pb.TripStatus_TRIP_STATUS_ACCEPTED, pb.TripStatus_TRIP_STATUS_ARRIVED:
	case pb.TripStatus_TRIP_STATUS_IN_TRIP:
		target, phase = trip.Destination, "destination"
	default:
		return nil, status.Error(codes.FailedPrecondition, "navigation requires an active assigned trip")
	}
	route, err := h.router.Fetch(ctx, req.Origin, target)
	if err != nil {
		return nil, err
	}
	return &pb.NavigationResponse{TripId: trip.TripId, Phase: phase, Route: route}, nil
}
