package config

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
)

// Config contains configuration for GraphQL Gateway
type Config struct {
	HTTPAddr                string
	AuthServiceURL          string
	UserServiceURL          string
	WalletServiceURL        string
	MediaServiceURL         string
	ChainRegistryServiceURL string
	OrchestratorServiceURL  string
	SubscriptionWorkerWSURL string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	log.Println("Loading GraphQL Gateway configuration...")

	config := &Config{
		HTTPAddr:                env.GetString("GATEWAY_HTTP_ADDR", ":8081"),
		AuthServiceURL:          env.GetString("AUTH_SERVICE_URL", "auth-service:50051"),
		UserServiceURL:          env.GetString("USER_SERVICE_URL", "user-service:50052"),
		WalletServiceURL:        env.GetString("WALLET_SERVICE_URL", "wallet-service:50053"),
		MediaServiceURL:         env.GetString("MEDIA_SERVICE_URL", "media-service:50055"),
		ChainRegistryServiceURL: env.GetString("CHAIN_REGISTRY_SERVICE_URL", "chain-registry-service:50056"),
		OrchestratorServiceURL:  env.GetString("ORCHESTRATOR_SERVICE_URL", "orchestrator-service:50054"),
		SubscriptionWorkerWSURL: env.GetString("SUBSCRIPTION_WORKER_WS_URL", "ws://subscription-worker:8080/ws"),
	}

	log.Printf("GraphQL Gateway config loaded - HTTP: %s, Orchestrator: %s",
		config.HTTPAddr, config.OrchestratorServiceURL)

	return config
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.HTTPAddr == "" {
		log.Fatal("GATEWAY_HTTP_ADDR is required")
	}
	if c.AuthServiceURL == "" {
		log.Fatal("AUTH_SERVICE_URL is required")
	}
	if c.UserServiceURL == "" {
		log.Fatal("USER_SERVICE_URL is required")
	}
	if c.WalletServiceURL == "" {
		log.Fatal("WALLET_SERVICE_URL is required")
	}

	if c.MediaServiceURL == "" {
		log.Fatal("MEDIA_SERVICE_URL is required")
	}
	if c.ChainRegistryServiceURL == "" {
		log.Fatal("CHAIN_REGISTRY_SERVICE_URL is required")
	}
	if c.OrchestratorServiceURL == "" {
		log.Fatal("ORCHESTRATOR_SERVICE_URL is required")
	}

	log.Println("GraphQL Gateway configuration validation passed")
	return nil
}
