package grpcclients

import (
	"fmt"

	"github.com/quangdang46/NFT-Marketplace/shared/logging"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/media"
	"github.com/quangdang46/NFT-Marketplace/shared/resilience"
)

type MediaClient struct {
	Client          *media.MediaServiceClient
	resilientClient *ResilientClient
}

func NewMediaClient(url string) *MediaClient {
	logger := logging.NewLogger(logging.DefaultConfig("graphql-gateway"))

	resilientClient, err := NewResilientClient("media-service", url, logger)
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"url":   url,
		}).Fatal("Failed to create media client")
		panic(fmt.Sprintf("failed to create media client: %v", err))
	}

	client := media.NewMediaServiceClient(resilientClient.GetConnection())

	return &MediaClient{
		Client:          &client,
		resilientClient: resilientClient,
	}
}

// GetStats returns circuit breaker statistics for the media service
func (c *MediaClient) GetStats() resilience.CircuitBreakerStats {
	return c.resilientClient.GetStats()
}

// Close closes the underlying connection
func (c *MediaClient) Close() error {
	return c.resilientClient.Close()
}
