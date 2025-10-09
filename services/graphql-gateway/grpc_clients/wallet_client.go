package grpcclients

import (
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/shared/logging"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"github.com/quangdang46/NFT-Marketplace/shared/resilience"
)

type WalletClient struct {
	Client          *wallet.WalletServiceClient
	resilientClient *ResilientClient
}

func NewWalletClient(url string) *WalletClient {
	logger := logging.NewLogger(logging.DefaultConfig("graphql-gateway"))

	resilientClient, err := NewResilientClient("wallet-service", url, logger)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Fatal("Failed to create wallet client")
		panic(fmt.Sprintf("failed to create wallet client: %v", err))
	}

	client := wallet.NewWalletServiceClient(resilientClient.GetConnection())

	return &WalletClient{
		Client:          &client,
		resilientClient: resilientClient,
	}
}

// GetStats returns circuit breaker statistics for the wallet service
func (c *WalletClient) GetStats() resilience.CircuitBreakerStats {
	return c.resilientClient.GetStats()
}

// Close closes the underlying connection
func (c *WalletClient) Close() error {
	return c.resilientClient.Close()
}
