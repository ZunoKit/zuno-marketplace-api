package config

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

// Config contains configuration for Orchestrator Service
type Config struct {
	GRPCPort             string
	Postgres             postgres.PostgresConfig
	Redis                redis.RedisConfig
	ChainRegistryGRPCURL string
	Features             Features
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	log.Println("Loading Orchestrator Service configuration...")

	c := &Config{
		GRPCPort:             env.GetString("ORCHESTRATOR_GRPC_PORT", ":50054"),
		Postgres:             loadPostgresConfig(),
		Redis:                loadRedisConfig(),
		ChainRegistryGRPCURL: env.GetString("CHAIN_REGISTRY_URL", "localhost:50056"),
		Features:             loadFeatures(),
	}

	log.Printf("Orchestrator config loaded - grpc=%s chain-registry=%s", c.GRPCPort, c.ChainRegistryGRPCURL)
	return c
}

type Features struct {
	SessionLinkedIntents       bool
	SessionValidationTimeoutMs int
}

func loadFeatures() Features {
	return Features{
		SessionLinkedIntents:       env.GetBool("SESSION_LINKED_INTENTS", false),
		SessionValidationTimeoutMs: env.GetInt("SESSION_VALIDATION_TIMEOUT_MS", 5000),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GRPCPort == "" {
		log.Fatal("GRPC_PORT is required")
	}
	if c.Postgres.PostgresHost == "" {
		log.Fatal("POSTGRES_HOST is required")
	}
	if c.Redis.RedisHost == "" {
		log.Fatal("REDIS_HOST is required")
	}
	if c.ChainRegistryGRPCURL == "" {
		log.Fatal("CHAIN_REGISTRY_URL is required")
	}
	log.Println("Orchestrator Service configuration validation passed")
	return nil
}

func loadPostgresConfig() postgres.PostgresConfig {
	return postgres.PostgresConfig{
		PostgresHost:     env.GetString("POSTGRES_HOST", "localhost"),
		PostgresPort:     env.GetInt("POSTGRES_PORT", 5432),
		PostgresUser:     env.GetString("POSTGRES_USER", "postgres"),
		PostgresPassword: env.GetString("POSTGRES_PASSWORD", "postgres"),
		PostgresDatabase: env.GetString("POSTGRES_DATABASE", "nft_marketplace"),
	}
}

func loadRedisConfig() redis.RedisConfig {
	return redis.RedisConfig{
		RedisHost: env.GetString("REDIS_HOST", "localhost"),
		RedisPort: env.GetInt("REDIS_PORT", 6379),
	}
}
