package trip_client

import (
	"context"
	"os"

	pb "github.com/sikozonpc/ride-sharing/shared/proto/trip"
	"github.com/sikozonpc/ride-sharing/shared/rpcsecurity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TripServiceClient struct {
	Client pb.TripServiceClient
	conn   *grpc.ClientConn
}

func NewTripServiceClient() (*TripServiceClient, error) {
	tripServiceURL := os.Getenv("TRIP_SERVICE_URL")
	if tripServiceURL == "" {
		tripServiceURL = "trip-service:9093"
	}

	options, err := rpcsecurity.ClientOptions()
	if err != nil {
		return nil, err
	}
	options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.NewClient(tripServiceURL, options...)
	if err != nil {
		return nil, err
	}

	client := pb.NewTripServiceClient(conn)
	return &TripServiceClient{
		Client: client,
		conn:   conn,
	}, nil
}

func (ds *TripServiceClient) Close() {
	if ds.conn != nil {
		err := ds.conn.Close()
		if err != nil {
			return
		}
	}
}

func (ds *TripServiceClient) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {
	return ds.Client.CreateTrip(ctx, req)
}

func (ds *TripServiceClient) AcceptTrip(ctx context.Context, req *pb.AcceptTripRequest) (*pb.AcceptTripResponse, error) {
	return ds.Client.AcceptTrip(ctx, req)
}

func (ds *TripServiceClient) GetTrip(ctx context.Context, req *pb.GetTripRequest) (*pb.GetTripResponse, error) {
	return ds.Client.GetTrip(ctx, req)
}
