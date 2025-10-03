package grpcclients

import (
	"log"

	chainregpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ChainRegistryClient struct {
	Client *chainregpb.ChainRegistryServiceClient
	conn   *grpc.ClientConn
}

func NewChainRegistryClient(url string) *ChainRegistryClient {
	dialOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		log.Fatalf("failed to dial chain-registry service: %v", err)
	}
	client := chainregpb.NewChainRegistryServiceClient(conn)
	return &ChainRegistryClient{Client: &client, conn: conn}
}
