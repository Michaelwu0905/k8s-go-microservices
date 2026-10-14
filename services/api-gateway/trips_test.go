package main

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTripErrorHTTPStatus(t *testing.T) {
	for _, tc := range []struct {
		code codes.Code
		http int
	}{
		{codes.InvalidArgument, 400}, {codes.NotFound, 404}, {codes.PermissionDenied, 403},
		{codes.AlreadyExists, 409}, {codes.FailedPrecondition, 409}, {codes.Unavailable, 503}, {codes.DeadlineExceeded, 504},
	} {
		w := httptest.NewRecorder()
		writeTripError(w, status.Error(tc.code, "test"))
		if w.Code != tc.http {
			t.Errorf("%v: got %d", tc.code, w.Code)
		}
	}
}

func TestTripRequestBodyValidation(t *testing.T) {
	for _, body := range []string{`{"driver_id":"a"} {}`, `{"unknown":1}`, `null {}`, `{`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/trip/accept", strings.NewReader(body))
		var target struct {
			DriverID string `json:"driver_id"`
		}
		if decodeTripBody(w, r, &target) {
			t.Errorf("accepted invalid body %s", body)
		}
	}
}
