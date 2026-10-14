package main

import (
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func TestDriverStateMachine(t *testing.T) {
	allowed := map[[2]pb.TripStatus]bool{
		{2, 2}: true, {2, 3}: true, {2, 6}: true,
		{3, 3}: true, {3, 4}: true, {3, 6}: true,
		{4, 4}: true, {4, 5}: true,
		{5, 5}: true, {6, 6}: true,
	}
	for from := pb.TripStatus(2); from <= 6; from++ {
		for to := pb.TripStatus(0); to <= 7; to++ {
			err := checkTransition(&pb.Trip{Status: from, DriverId: "driver-0", RiderId: "rider-0"}, to, "driver-0", "")
			if (err == nil) != allowed[[2]pb.TripStatus{from, to}] {
				t.Errorf("%s -> %s: %v", from, to, err)
			}
		}
	}
}

func TestTripActorPermissions(t *testing.T) {
	trip := &pb.Trip{Status: 2, DriverId: "driver-0", RiderId: "rider-0"}
	for _, tc := range []struct {
		driver, rider string
		next          pb.TripStatus
		code          codes.Code
	}{
		{"driver-1", "", 3, codes.PermissionDenied},
		{"", "rider-1", 6, codes.PermissionDenied},
		{"", "rider-0", 3, codes.PermissionDenied},
		{"", "rider-0", 6, codes.OK},
	} {
		if got := status.Code(checkTransition(trip, tc.next, tc.driver, tc.rider)); got != tc.code {
			t.Errorf("%+v: %s", tc, got)
		}
	}
	trip.Status = 4
	if status.Code(checkTransition(trip, 6, "", "rider-0")) != codes.FailedPrecondition {
		t.Fatal("rider cancelled an in-progress trip")
	}
}
