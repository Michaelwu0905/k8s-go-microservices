package main

import (
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/protobuf/proto"
	"net/http"
)

func requireRole(w http.ResponseWriter, r *http.Request, role string) bool {
	user := currentUser(r)
	if user == nil {
		authError(w, 401, "请先登录")
		return false
	}
	if user.Role != role {
		authError(w, 403, "当前身份无权执行此操作")
		return false
	}
	return true
}
func claimsMatch(w http.ResponseWriter, r *http.Request, riderID, driverID string) bool {
	user := currentUser(r)
	if user == nil {
		authError(w, 401, "请先登录")
		return false
	}
	if riderID != "" && (user.Role != "rider" || riderID != user.ID) || driverID != "" && (user.Role != "driver" || driverID != user.ID) {
		authError(w, 403, "不能使用其他用户的身份")
		return false
	}
	return true
}
func canViewTrip(user *Principal, trip *pb.Trip) bool {
	return user != nil && (user.Role == "rider" && trip.RiderId == user.ID || user.Role == "driver" && trip.DriverId == user.ID)
}
func authorizeTrip(w http.ResponseWriter, r *http.Request, trip *pb.Trip) bool {
	if !canViewTrip(currentUser(r), trip) {
		authError(w, 403, "无权访问此订单")
		return false
	}
	return true
}
func ownListFilter(w http.ResponseWriter, r *http.Request) (*pb.ListTripsRequest, bool) {
	user := currentUser(r)
	if user == nil {
		authError(w, 401, "请先登录")
		return nil, false
	}
	if !claimsMatch(w, r, r.URL.Query().Get("rider_id"), r.URL.Query().Get("driver_id")) {
		return nil, false
	}
	if user.Role == "rider" {
		return &pb.ListTripsRequest{RiderId: user.ID}, true
	}
	return &pb.ListTripsRequest{DriverId: user.ID}, true
}
func driverOwnsPath(w http.ResponseWriter, r *http.Request) bool {
	if !requireRole(w, r, "driver") {
		return false
	}
	if currentUser(r).ID != r.PathValue("id") {
		authError(w, 403, "不能操作其他司机的位置")
		return false
	}
	return true
}
func sanitizeTrips(user *Principal, trips []*pb.Trip) []*pb.Trip {
	result := make([]*pb.Trip, 0, len(trips))
	for _, trip := range trips {
		if canViewTrip(user, trip) {
			result = append(result, trip)
			continue
		}
		if user.Role == "driver" && trip.Status == pb.TripStatus_TRIP_STATUS_PENDING && trip.DriverId == "" {
			offer := proto.Clone(trip).(*pb.Trip)
			offer.RiderId = ""
			result = append(result, offer)
		}
	}
	return result
}
func assignedDriver(r *http.Request, trips *trip_client.TripServiceClient) (string, error) {
	list, err := trips.Client.ListTrips(r.Context(), &pb.ListTripsRequest{RiderId: currentUser(r).ID})
	if err != nil {
		return "", err
	}
	for _, trip := range list.Trips {
		if trip.Status >= 2 && trip.Status <= 4 {
			return trip.DriverId, nil
		}
	}
	return "", nil
}
