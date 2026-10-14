package main

import (
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"regexp"
)

var identityPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validIdentity(id string) bool { return identityPattern.MatchString(id) }

func checkTransition(t *pb.Trip, next pb.TripStatus, driverID, riderID string) error {
	if driverID != "" {
		if driverID != t.DriverId {
			return status.Error(codes.PermissionDenied, "only the assigned driver can update this trip")
		}
	} else if riderID == "" || riderID != t.RiderId || next != pb.TripStatus_TRIP_STATUS_CANCELLED {
		return status.Error(codes.PermissionDenied, "riders can only cancel their own trips")
	}
	if next == t.Status {
		return nil
	}
	allowed := false
	switch t.Status {
	case pb.TripStatus_TRIP_STATUS_PENDING:
		allowed = next == pb.TripStatus_TRIP_STATUS_CANCELLED
	case pb.TripStatus_TRIP_STATUS_ACCEPTED:
		allowed = next == pb.TripStatus_TRIP_STATUS_ARRIVED || next == pb.TripStatus_TRIP_STATUS_CANCELLED
	case pb.TripStatus_TRIP_STATUS_ARRIVED:
		allowed = next == pb.TripStatus_TRIP_STATUS_IN_TRIP || next == pb.TripStatus_TRIP_STATUS_CANCELLED
	case pb.TripStatus_TRIP_STATUS_IN_TRIP:
		allowed = next == pb.TripStatus_TRIP_STATUS_COMPLETED
	}
	if !allowed {
		return status.Errorf(codes.FailedPrecondition, "cannot transition from %s to %s", t.Status, next)
	}
	return nil
}
