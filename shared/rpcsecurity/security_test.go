package rpcsecurity

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
)

func TestInternalCredentialsProtectUnaryAndStream(t *testing.T) {
	key := "test-internal-key-at-least-32-bytes-long"
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(serverOptions(key)...)
	healthpb.RegisterHealthServer(server, health.NewServer())
	go server.Serve(listener)
	defer server.Stop()
	client, err := grpc.NewClient("passthrough:///internal", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	healthClient := healthpb.NewHealthClient(client)
	if _, err := healthClient.Check(context.Background(), &healthpb.HealthCheckRequest{}); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unprotected unary: %v", err)
	}
	stream, err := healthClient.Watch(context.Background(), &healthpb.HealthCheckRequest{})
	if err == nil {
		_, err = stream.Recv()
	}
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unprotected stream: %v", err)
	}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs(header, key))
	if _, err := healthClient.Check(ctx, &healthpb.HealthCheckRequest{}); err != nil {
		t.Fatal(err)
	}
	stream, err = healthClient.Watch(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.Recv(); err != nil {
		t.Fatal(err)
	}
}
