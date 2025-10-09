package grpcclients

import (
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/shared/logging"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	"github.com/quangdang46/NFT-Marketplace/shared/resilience"
)

type AuthClient struct {
	Client          *auth.AuthServiceClient
	resilientClient *ResilientClient
}

func NewAuthClient(url string) *AuthClient {
	logger := logging.NewLogger(logging.DefaultConfig("graphql-gateway"))

	resilientClient, err := NewResilientClient("auth-service", url, logger)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Fatal("Failed to create auth client")
		panic(fmt.Sprintf("failed to create auth client: %v", err))
	}

	client := auth.NewAuthServiceClient(resilientClient.GetConnection())

	return &AuthClient{
		Client:          &client,
		resilientClient: resilientClient,
	}
}

// GetStats returns circuit breaker statistics for the auth service
func (c *AuthClient) GetStats() resilience.CircuitBreakerStats {
	return c.resilientClient.GetStats()
}

// Close closes the underlying connection
func (c *AuthClient) Close() error {
	return c.resilientClient.Close()
}
