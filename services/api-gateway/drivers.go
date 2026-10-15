package main

import (
	"context"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/driver_client"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"
	driverpb "github.com/sikozonpc/ride-sharing/shared/proto/driver"
	trippb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"time"
)

func registerDriverRoutes(mux *http.ServeMux, drivers *driver_client.DriverServiceClient, trips *trip_client.TripServiceClient) {
	register := func(path, method string, handler http.HandlerFunc) {
		mux.HandleFunc(path, enableCORS(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method {
				w.Header().Set("Allow", method)
				http.Error(w, "Method not allowed", 405)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			handler(w, r.WithContext(ctx))
		}))
	}
	register("/drivers/{id}", http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		driver, err := drivers.Client.GetDriver(r.Context(), &driverpb.GetDriverRequest{DriverId: r.PathValue("id")})
		if err != nil {
			writeTripError(w, err)
			return
		}
		_ = writeJSON(w, http.StatusOK, driver)
	})
	register("/drivers/{id}/availability", http.MethodPut, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Online *bool `json:"online"`
		}
		if !decodeTripBody(w, r, &body) {
			return
		}
		if body.Online == nil {
			writeTripError(w, status.Error(codes.InvalidArgument, "online is required"))
			return
		}
		driver, err := drivers.Client.SetAvailability(r.Context(), &driverpb.SetAvailabilityRequest{DriverId: r.PathValue("id"), Online: *body.Online})
		if err != nil {
			writeTripError(w, err)
			return
		}
		_ = writeJSON(w, http.StatusOK, driver)
	})
	register("/drivers/{id}/location", http.MethodPut, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Location *driverpb.Location `json:"location"`
			Source   string             `json:"source"`
			Accuracy float32            `json:"accuracy"`
		}
		if !decodeTripBody(w, r, &body) {
			return
		}
		driver, err := drivers.Client.UpdateLocation(r.Context(), &driverpb.UpdateLocationRequest{DriverId: r.PathValue("id"), Location: body.Location, Source: body.Source, Accuracy: body.Accuracy})
		if err != nil {
			writeTripError(w, err)
			return
		}
		_ = writeJSON(w, http.StatusOK, driver)
	})
	register("/trip/{id}/navigation", http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		trip, err := trips.GetTrip(r.Context(), &trippb.GetTripRequest{TripId: r.PathValue("id")})
		if err != nil {
			writeTripError(w, err)
			return
		}
		if trip.Trip.DriverId == "" {
			writeTripError(w, status.Error(codes.FailedPrecondition, "trip has no assigned driver"))
			return
		}
		driver, err := drivers.Client.GetDriver(r.Context(), &driverpb.GetDriverRequest{DriverId: trip.Trip.DriverId})
		if err != nil {
			writeTripError(w, err)
			return
		}
		if driver.Location == nil || !driver.Online {
			writeTripError(w, status.Error(codes.FailedPrecondition, "driver location is offline or expired"))
			return
		}
		navigation, err := trips.Client.GetNavigation(r.Context(), &trippb.GetNavigationRequest{
			TripId: trip.TripId, DriverId: driver.DriverId, Origin: &trippb.Coordinate{Latitude: driver.Location.Latitude, Longitude: driver.Location.Longitude},
		})
		if err != nil {
			writeTripError(w, err)
			return
		}
		_ = writeJSON(w, http.StatusOK, map[string]any{"trip_id": navigation.TripId, "phase": navigation.Phase, "route": navigation.Route, "driver": driver})
	})
}
