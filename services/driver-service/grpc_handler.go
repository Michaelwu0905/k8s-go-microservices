package main

import (
	"context"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/driver"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"math"
	"time"
)

type DriverGrpcHandler struct {
	pb.UnimplementedDriverServiceServer
	service *Service
}

func NewGrpcHandler(server *grpc.Server, service *Service) {
	pb.RegisterDriverServiceServer(server, &DriverGrpcHandler{service: service})
}

func (h *DriverGrpcHandler) UpdateLocation(ctx context.Context, req *pb.UpdateLocationRequest) (*pb.Driver, error) {
	if !driverIdentity.MatchString(req.GetDriverId()) || !validLocation(req.GetLocation()) ||
		(req.GetSource() != "gps" && req.GetSource() != "demo") || math.IsNaN(float64(req.GetAccuracy())) || math.IsInf(float64(req.GetAccuracy()), 0) || req.GetAccuracy() < 0 {
		return nil, status.Error(codes.InvalidArgument, "valid driver_id, location, source (gps/demo) and accuracy are required")
	}
	return h.service.UpdateLocation(ctx, req)
}
func (h *DriverGrpcHandler) SetAvailability(ctx context.Context, req *pb.SetAvailabilityRequest) (*pb.Driver, error) {
	if !driverIdentity.MatchString(req.GetDriverId()) {
		return nil, status.Error(codes.InvalidArgument, "valid driver_id is required")
	}
	return h.service.SetAvailability(ctx, req.DriverId, req.Online)
}
func (h *DriverGrpcHandler) GetDriver(ctx context.Context, req *pb.GetDriverRequest) (*pb.Driver, error) {
	if !driverIdentity.MatchString(req.GetDriverId()) {
		return nil, status.Error(codes.InvalidArgument, "valid driver_id is required")
	}
	return h.service.Get(ctx, req.DriverId)
}
func (h *DriverGrpcHandler) FindNearbyDrivers(stream pb.DriverService_FindNearbyDriversServer) error {
	requests := make(chan *pb.FindNearbyDriversRequest, 1)
	failures := make(chan error, 1)
	go func() {
		for {
			req, err := stream.Recv()
			if err != nil {
				failures <- err
				return
			}
			select {
			case requests <- req:
			case <-stream.Context().Done():
				return
			}
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var latest *pb.FindNearbyDriversRequest
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case err := <-failures:
			if err == io.EOF {
				return nil
			}
			return err
		case req := <-requests:
			if !validLocation(req.GetLocation()) || (req.AssignedDriverId != "" && !driverIdentity.MatchString(req.AssignedDriverId)) {
				return status.Error(codes.InvalidArgument, "valid location is required")
			}
			latest = req
		case <-ticker.C:
			if latest == nil {
				continue
			}
		}
		if latest == nil {
			continue
		}
		ctx, cancel := context.WithTimeout(stream.Context(), 3*time.Second)
		drivers, err := h.service.FindNearbyDrivers(ctx, latest.Location, latest.AssignedDriverId)
		cancel()
		if err != nil {
			return err
		}
		if err := stream.Send(&pb.StreamDriversResponse{NearbyDrivers: drivers}); err != nil {
			return err
		}
	}
}
