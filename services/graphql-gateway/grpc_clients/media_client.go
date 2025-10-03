package grpcclients

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/proto/media"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MediaClient struct {
	Client *media.MediaServiceClient
	conn   *grpc.ClientConn
}

func NewMediaClient(url string) *MediaClient {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		log.Fatalf("failed to dial media service: %v", err)
	}

	client := media.NewMediaServiceClient(conn)

	return &MediaClient{
		Client: &client,
		conn:   conn,
	}
}

func (c *MediaClient) Close() error {
	return c.conn.Close()
}
