package grpcclients

import (
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/shared/logging"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
	"github.com/quangdang46/NFT-Marketplace/shared/resilience"
)

type OrchestratorClient struct {
	Client          *orchestratorpb.OrchestratorServiceClient
	resilientClient *ResilientClient
}

func NewOrchestratorClient(url string) *OrchestratorClient {
	logger := logging.NewLogger(logging.DefaultConfig("graphql-gateway"))

	resilientClient, err := NewResilientClient("orchestrator-service", url, logger)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Fatal("Failed to create orchestrator client")
		panic(fmt.Sprintf("failed to create orchestrator client: %v", err))
	}

	client := orchestratorpb.NewOrchestratorServiceClient(resilientClient.GetConnection())

	return &OrchestratorClient{
		Client:          &client,
		resilientClient: resilientClient,
	}
}

// GetStats returns circuit breaker statistics for the orchestrator service
func (c *OrchestratorClient) GetStats() resilience.CircuitBreakerStats {
	return c.resilientClient.GetStats()
}

// Close closes the underlying connection
func (c *OrchestratorClient) Close() error {
	return c.resilientClient.Close()
}
