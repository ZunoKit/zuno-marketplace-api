package config

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

// Config contains configuration for User Service
type Config struct {
	GRPCPort string
	Postgres postgres.PostgresConfig
	Redis    redis.RedisConfig
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	log.Println("Loading User Service configuration...")

	config := &Config{
		GRPCPort: env.GetString("USER_GRPC_PORT", ":50052"),
		Postgres: loadPostgresConfig(),
		Redis:    loadRedisConfig(),
	}

	log.Printf("User Service config loaded - gRPC: %s",
		config.GRPCPort)

	return config
}

// loadPostgresConfig loads PostgreSQL configuration
func loadPostgresConfig() postgres.PostgresConfig {
	return postgres.PostgresConfig{
		PostgresHost:     env.GetString("POSTGRES_HOST", "localhost"),
		PostgresPort:     env.GetInt("POSTGRES_PORT", 5432),
		PostgresUser:     env.GetString("POSTGRES_USER", "postgres"),
		PostgresPassword: env.GetString("POSTGRES_PASSWORD", "password"),
		PostgresDatabase: env.GetString("POSTGRES_DATABASE", "nft_marketplace"),
	}
}

// loadRedisConfig loads Redis configuration
func loadRedisConfig() redis.RedisConfig {
	return redis.RedisConfig{
		RedisHost: env.GetString("REDIS_HOST", "localhost"),
		RedisPort: env.GetInt("REDIS_PORT", 6379),
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

	log.Println("User Service configuration validation passed")
	return nil
}
