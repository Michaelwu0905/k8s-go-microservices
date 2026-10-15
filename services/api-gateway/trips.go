package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"io"
	"net/http"
	"time"
)

func writeTripError(w http.ResponseWriter, err error) {
	code := http.StatusInternalServerError
	switch status.Code(err) {
	case codes.Unauthenticated:
		code = http.StatusUnauthorized
	case codes.InvalidArgument:
		code = http.StatusBadRequest
	case codes.NotFound:
		code = http.StatusNotFound
	case codes.PermissionDenied:
		code = http.StatusForbidden
	case codes.AlreadyExists, codes.FailedPrecondition, codes.Aborted:
		code = http.StatusConflict
	case codes.Unavailable:
		code = http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		code = http.StatusGatewayTimeout
	}
	_ = writeJSON(w, code, map[string]string{"error": status.Convert(err).Message()})
}

func decodeTripBody(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		_ = writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON request"})
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		_ = writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected one JSON object"})
		return false
	}
	return true
}

func registerTripRoutes(mux *http.ServeMux, client *trip_client.TripServiceClient) {
	register := func(path, method string, handler http.HandlerFunc) {
		mux.HandleFunc(path, enableCORS(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method {
				w.Header().Set("Allow", method)
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			handler(w, r.WithContext(ctx))
		}))
	}
	register("/trip", http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, "rider") {
			return
		}
		var body struct {
			Pickup      *pb.Coordinate `json:"pickup"`
			Destination *pb.Coordinate `json:"destination"`
			RiderID     string         `json:"rider_id"`
		}
		if !decodeTripBody(w, r, &body) {
			return
		}
		if !claimsMatch(w, r, body.RiderID, "") {
			return
		}
		resp, err := client.CreateTrip(r.Context(), &pb.CreateTripRequest{StartLocation: body.Pickup, EndLocation: body.Destination, RiderId: currentUser(r).ID})
		if err != nil {
			writeTripError(w, err)
			return
		}
		_ = writeJSON(w, http.StatusCreated, resp.Trip)
	})
	register("/trip/accept", http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		if !requireRole(w, r, "driver") {
			return
		}
		var body struct {
			TripID   string `json:"trip_id"`
			DriverID string `json:"driver_id"`
		}
		if !decodeTripBody(w, r, &body) {
			return
		}
		if !claimsMatch(w, r, "", body.DriverID) {
			return
		}
		resp, err := client.AcceptTrip(r.Context(), &pb.AcceptTripRequest{TripId: body.TripID, DriverId: currentUser(r).ID})
		if err != nil {
			writeTripError(w, err)
			return
		}
		_ = writeJSON(w, http.StatusOK, resp.Trip)
	})
	register("/trip/{id}", http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		resp, err := client.GetTrip(r.Context(), &pb.GetTripRequest{TripId: r.PathValue("id")})
		if err != nil {
			writeTripError(w, err)
			return
		}
		if !authorizeTrip(w, r, resp.Trip) {
			return
		}
		_ = writeJSON(w, http.StatusOK, resp.Trip)
	})
	register("/trips", http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		filter, ok := ownListFilter(w, r)
		if !ok {
			return
		}
		resp, err := client.Client.ListTrips(r.Context(), filter)
		if err != nil {
			writeTripError(w, err)
			return
		}
		resp.Trips = sanitizeTrips(currentUser(r), resp.Trips)
		writeTripList(w, resp)
	})
	register("/trip/{id}/status", http.MethodPatch, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Status   pb.TripStatus `json:"status"`
			DriverID string        `json:"driver_id"`
			RiderID  string        `json:"rider_id"`
		}
		if !decodeTripBody(w, r, &body) {
			return
		}
		if !claimsMatch(w, r, body.RiderID, body.DriverID) {
			return
		}
		user := currentUser(r)
		update := &pb.UpdateTripStatusRequest{TripId: r.PathValue("id"), Status: body.Status}
		if user.Role == "rider" {
			update.RiderId = user.ID
		} else {
			update.DriverId = user.ID
		}
		resp, err := client.Client.UpdateTripStatus(r.Context(), update)
		if err != nil {
			writeTripError(w, err)
			return
		}
		_ = writeJSON(w, http.StatusOK, resp.Trip)
	})
	mux.HandleFunc("/events/trips", enableCORS(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}
		filter, ok := ownListFilter(w, r)
		if !ok {
			return
		}
		request := &pb.WatchTripsRequest{RiderId: filter.RiderId, DriverId: filter.DriverId}
		if tripID := r.URL.Query().Get("trip_id"); tripID != "" {
			trip, err := client.GetTrip(r.Context(), &pb.GetTripRequest{TripId: tripID})
			if err != nil {
				writeTripError(w, err)
				return
			}
			if !authorizeTrip(w, r, trip.Trip) {
				return
			}
			request = &pb.WatchTripsRequest{TripId: tripID}
		}
		stream, err := client.Client.WatchTrips(r.Context(), request)
		if err != nil {
			writeTripError(w, err)
			return
		}
		event, err := stream.Recv()
		if err != nil {
			writeTripError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		for {
			event.Trips = sanitizeTrips(currentUser(r), event.Trips)
			payload, err := json.Marshal(map[string]any{"trips": event.Trips})
			if err != nil {
				return
			}
			_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(30 * time.Second))
			if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
				return
			}
			flusher.Flush()
			event, err = stream.Recv()
			if err != nil {
				return
			}
		}
	}))
}

func writeTripList(w http.ResponseWriter, resp *pb.ListTripsResponse) {
	if resp.Trips == nil {
		resp.Trips = []*pb.Trip{}
	}
	_ = writeJSON(w, http.StatusOK, map[string]any{"trips": resp.Trips})
}
