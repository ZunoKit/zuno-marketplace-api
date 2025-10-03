package grpcclients

import (
	"log"

	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OrchestratorClient struct {
	Client *orchestratorpb.OrchestratorServiceClient
	conn   *grpc.ClientConn
}

func NewOrchestratorClient(url string) *OrchestratorClient {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		log.Fatalf("failed to dial orchestrator service: %v", err)
	}

	client := orchestratorpb.NewOrchestratorServiceClient(conn)

	return &OrchestratorClient{
		Client: &client,
		conn:   conn,
	}
}
