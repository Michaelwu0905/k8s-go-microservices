// Package rpcsecurity restricts internal gRPC endpoints to trusted gateway calls.
package rpcsecurity

import (
	"context"
	"crypto/subtle"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"os"
)

const header = "x-internal-service-key"

func configuredKey() (string, error) {
	key := os.Getenv("INTERNAL_RPC_KEY")
	if len(key) < 32 {
		return "", fmt.Errorf("INTERNAL_RPC_KEY must contain at least 32 bytes")
	}
	return key, nil
}
func ClientOptions() ([]grpc.DialOption, error) {
	key, err := configuredKey()
	if err != nil {
		return nil, err
	}
	attach := func(ctx context.Context) context.Context { return metadata.AppendToOutgoingContext(ctx, header, key) }
	return []grpc.DialOption{
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoke(attach(ctx), method, req, reply, cc, opts...)
		}),
		grpc.WithStreamInterceptor(func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, stream grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return stream(attach(ctx), desc, cc, method, opts...)
		}),
	}, nil
}
func ServerOptions() ([]grpc.ServerOption, error) {
	key, err := configuredKey()
	if err != nil {
		return nil, err
	}
	return serverOptions(key), nil
}
func serverOptions(key string) []grpc.ServerOption {
	authorize := func(ctx context.Context) error {
		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get(header)
		if len(values) != 1 || subtle.ConstantTimeCompare([]byte(values[0]), []byte(key)) != 1 {
			return status.Error(codes.Unauthenticated, "trusted service credentials required")
		}
		return nil
	}
	return []grpc.ServerOption{
		grpc.UnaryInterceptor(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
			if err := authorize(ctx); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(service any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			if err := authorize(stream.Context()); err != nil {
				return err
			}
			return handler(service, stream)
		}),
	}
}
