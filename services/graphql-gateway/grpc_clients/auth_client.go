package grpcclients

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthClient struct {
	Client *auth.AuthServiceClient
	conn   *grpc.ClientConn
}

func NewAuthClient(url string) *AuthClient {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		log.Fatalf("failed to dial auth service: %v", err)
	}

	client := auth.NewAuthServiceClient(conn)

	return &AuthClient{
		Client: &client,
		conn:   conn,
	}
}
