package grpcclients

import (
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/shared/logging"
	chainregpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"github.com/quangdang46/NFT-Marketplace/shared/resilience"
)

type ChainRegistryClient struct {
	Client          *chainregpb.ChainRegistryServiceClient
	resilientClient *ResilientClient
}

func NewChainRegistryClient(url string) *ChainRegistryClient {
	logger := logging.NewLogger(logging.DefaultConfig("graphql-gateway"))

	resilientClient, err := NewResilientClient("chain-registry-service", url, logger)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Fatal("Failed to create chain registry client")
		panic(fmt.Sprintf("failed to create chain registry client: %v", err))
	}

	client := chainregpb.NewChainRegistryServiceClient(resilientClient.GetConnection())

	return &ChainRegistryClient{
		Client:          &client,
		resilientClient: resilientClient,
	}
}

// GetStats returns circuit breaker statistics for the chain registry service
func (c *ChainRegistryClient) GetStats() resilience.CircuitBreakerStats {
	return c.resilientClient.GetStats()
}

// Close closes the underlying connection
func (c *ChainRegistryClient) Close() error {
	return c.resilientClient.Close()
}
