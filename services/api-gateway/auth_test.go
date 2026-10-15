package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sikozonpc/ride-sharing/services/api-gateway/grpc_clients/trip_client"
	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
)

func TestPermissionBoundaries(t *testing.T) {
	owner := &Principal{ID: "rider-owner", Role: "rider"}
	driver := &Principal{ID: "driver-owner", Role: "driver"}
	trip := &pb.Trip{RiderId: owner.ID, DriverId: driver.ID, Status: 2}
	for _, user := range []*Principal{owner, driver} {
		if !canViewTrip(user, trip) {
			t.Fatal("owner denied")
		}
	}
	if canViewTrip(&Principal{ID: "rider-other", Role: "rider"}, trip) || canViewTrip(nil, trip) {
		t.Fatal("foreign trip exposed")
	}
	req := httptest.NewRequest("POST", "/trip", nil).WithContext(context.WithValue(context.Background(), principalKey{}, owner))
	if claimsMatch(httptest.NewRecorder(), req, "rider-other", "") {
		t.Fatal("forged rider accepted")
	}
	if claimsMatch(httptest.NewRecorder(), req, "", "driver-owner") {
		t.Fatal("role escalation accepted")
	}
	if requireRole(httptest.NewRecorder(), req, "driver") {
		t.Fatal("driver endpoint exposed to rider")
	}
	pending := &pb.Trip{TripId: "offer", Status: 1, RiderId: "private-rider"}
	offers := sanitizeTrips(driver, []*pb.Trip{pending, trip, {TripId: "foreign", Status: 2, DriverId: "driver-other"}})
	if len(offers) != 2 || offers[0].RiderId != "" || pending.RiderId != "private-rider" {
		t.Fatal("offer redaction failed")
	}
}

type authTripClient struct {
	pb.TripServiceClient
	trip *pb.Trip
}

func (f *authTripClient) GetTrip(ctx context.Context, req *pb.GetTripRequest, opts ...grpc.CallOption) (*pb.GetTripResponse, error) {
	return &pb.GetTripResponse{Trip: f.trip}, nil
}
func (f *authTripClient) ListTrips(ctx context.Context, req *pb.ListTripsRequest, opts ...grpc.CallOption) (*pb.ListTripsResponse, error) {
	return &pb.ListTripsResponse{Trips: []*pb.Trip{f.trip}}, nil
}

func testAuthStore(t *testing.T) *AuthStore {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("set TEST_DATABASE_URL for authentication integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("auth_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{name}.Sanitize()+" CASCADE")
		admin.Close(context.Background())
	})
	parsed, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	params := parsed.Query()
	params.Set("search_path", name)
	parsed.RawQuery = params.Encode()
	store, err := NewAuthStore(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.pool.Close)
	return store
}
func TestAuthSessionsCSRFAndOwnership(t *testing.T) {
	store := testAuthStore(t)
	mux := http.NewServeMux()
	store.registerRoutes(mux, nil)
	fake := &authTripClient{}
	registerTripRoutes(mux, &trip_client.TripServiceClient{Client: fake})
	server := httptest.NewServer(store.middleware(mux))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	request := func(method, path, body, csrf, origin string) *http.Response {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	expect := func(resp *http.Response, code int) {
		t.Helper()
		defer resp.Body.Close()
		if resp.StatusCode != code {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected %d, got %d: %s", code, resp.StatusCode, body)
		}
	}
	expect(request("GET", "/trips", "", "", ""), 401)
	expect(request("POST", "/auth/register", `{"username":"admin1","password":"password-secure","display_name":"admin","role":"admin"}`, "", ""), 400)
	resp := request("POST", "/auth/register", `{"username":"passenger1","password":"password-secure","display_name":"乘客","role":"rider"}`, "", "http://localhost:3000")
	if resp.StatusCode != 201 {
		t.Fatalf("register: %d", resp.StatusCode)
	}
	var user Principal
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	cookies := resp.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode || cookies[0].MaxAge != int(sessionLifetime.Seconds()) {
		t.Fatal("unsafe session cookie")
	}
	if len(user.CSRFToken) != 43 || user.Role != "rider" {
		t.Fatal("invalid account session")
	}
	var hash []byte
	if err := store.pool.QueryRow(context.Background(), "SELECT password_hash FROM accounts WHERE id=$1", user.ID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if string(hash) == "password-secure" || bcrypt.CompareHashAndPassword(hash, []byte("password-secure")) != nil {
		t.Fatal("password was not hashed")
	}
	fake.trip = &pb.Trip{TripId: "owned", RiderId: user.ID, Status: 1}
	expect(request("GET", "/auth/me", "", "", ""), 200)
	expect(request("GET", "/trip/owned", "", "", ""), 200)
	fake.trip.RiderId = "someone-else"
	expect(request("GET", "/trip/owned", "", "", ""), 403)
	expect(request("GET", "/events/trips?trip_id=foreign", "", "", ""), 403)
	expect(request("GET", "/trips?rider_id=someone-else", "", "", ""), 403)
	expect(request("GET", "/trips?driver_id=driver-forged", "", "", ""), 403)
	expect(request("POST", "/trip/accept", `{"trip_id":"owned"}`, user.CSRFToken, ""), 403)
	expect(request("POST", "/trip", `{"rider_id":"forged"}`, user.CSRFToken, ""), 403)
	expect(request("POST", "/auth/logout", "", "", ""), 403)
	expect(request("POST", "/auth/logout", "", user.CSRFToken, "https://evil.example"), 403)
	cors := request("OPTIONS", "/trip", "", "", "http://localhost:3000")
	if cors.Header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" || cors.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentialed CORS missing")
	}
	expect(cors, 204)
	expect(request("POST", "/auth/logout", "", user.CSRFToken, "http://localhost:3000"), 200)
	replay := httptest.NewRequest("GET", "/auth/me", nil)
	replay.AddCookie(cookies[0])
	if _, err := store.authenticate(context.Background(), replay); err != pgx.ErrNoRows {
		t.Fatalf("logged-out session remained valid: %v", err)
	}
	expect(request("GET", "/auth/me", "", "", ""), 401)
	expect(request("POST", "/auth/login", `{"username":"passenger1","password":"wrong-password"}`, "", ""), 401)
	login := request("POST", "/auth/login", `{"username":"passenger1","password":"password-secure","role":"driver"}`, "", "")
	if err := json.NewDecoder(login.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
	login.Body.Close()
	if user.Role != "rider" {
		t.Fatal("login body changed role")
	}
	if _, err := store.pool.Exec(context.Background(), "UPDATE auth_sessions SET expires_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	expect(request("GET", "/auth/me", "", "", ""), 401)
}

func TestAuthRateLimitExpiresAndIgnoresForwardedIP(t *testing.T) {
	store := testAuthStore(t)
	r := httptest.NewRequest("POST", "/auth/login", nil)
	r.RemoteAddr = "192.0.2.10:1234"
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		allowed, err := store.allowAttempt(ctx, r, "credentials", "passenger1")
		if err != nil || !allowed {
			t.Fatalf("attempt %d: allowed=%v error=%v", i+1, allowed, err)
		}
	}
	r.Header.Set("X-Forwarded-For", "192.0.2.20")
	if allowed, err := store.allowAttempt(ctx, r, "credentials", "passenger1"); err != nil || allowed {
		t.Fatalf("forwarded header bypassed limit: allowed=%v error=%v", allowed, err)
	}
	if _, err := store.pool.Exec(ctx, "UPDATE auth_rate_limits SET expires_at=now()-interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if allowed, err := store.allowAttempt(ctx, r, "credentials", "passenger1"); err != nil || !allowed {
		t.Fatalf("expired limit did not reset: allowed=%v error=%v", allowed, err)
	}
}

func TestLogoutTerminatesLiveSession(t *testing.T) {
	store := testAuthStore(t)
	mux := http.NewServeMux()
	store.registerRoutes(mux, nil)
	mux.HandleFunc("/events/trips", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {}\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	server := httptest.NewServer(store.middleware(mux))
	defer server.Close()
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 6 * time.Second}
	register, err := client.Post(server.URL+"/auth/register", "application/json", strings.NewReader(`{"username":"streamuser","password":"password-secure","display_name":"stream","role":"rider"}`))
	if err != nil {
		t.Fatal(err)
	}
	var user Principal
	json.NewDecoder(register.Body).Decode(&user)
	register.Body.Close()
	stream, err := client.Get(server.URL + "/events/trips")
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	req, _ := http.NewRequest("POST", server.URL+"/auth/logout", nil)
	req.Header.Set("X-CSRF-Token", user.CSRFToken)
	logout, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	logout.Body.Close()
	if _, err := io.ReadAll(stream.Body); err != nil {
		t.Fatalf("stream did not close cleanly on logout: %v", err)
	}
}
