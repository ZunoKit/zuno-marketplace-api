package config

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
)

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Port string
}

// Config contains configuration for User Service
type Config struct {
	GRPCConfig     GRPCConfig
	PostgresConfig postgres.PostgresConfig
	RabbitMQ       messaging.RabbitMQConfig
}

// NewConfig creates and loads configuration from environment variables
func NewConfig() *Config {
	log.Println("Loading User Service configuration...")

	config := &Config{
		GRPCConfig: GRPCConfig{
			Port: env.GetString("USER_GRPC_PORT", ":50052"),
		},
		PostgresConfig: loadPostgresConfig(),
		RabbitMQ:       loadRabbitMQConfig(),
	}

	return config
}

// loadPostgresConfig loads PostgreSQL configuration
func loadPostgresConfig() postgres.PostgresConfig {
	return postgres.PostgresConfig{
		PostgresHost:     env.GetString("POSTGRES_HOST", "localhost"),
		PostgresPort:     env.GetInt("POSTGRES_PORT", 5432),
		PostgresUser:     env.GetString("POSTGRES_USER", "postgres"),
		PostgresPassword: env.GetString("POSTGRES_PASSWORD", "postgres"),
		PostgresDatabase: env.GetString("POSTGRES_DATABASE", "nft_marketplace"),
	}
}

// loadRabbitMQConfig loads RabbitMQ configuration
func loadRabbitMQConfig() messaging.RabbitMQConfig {
	return messaging.RabbitMQConfig{
		RabbitMQHost:     env.GetString("RABBITMQ_HOST", "localhost"),
		RabbitMQPort:     env.GetInt("RABBITMQ_PORT", 5671),
		RabbitMQUser:     env.GetString("RABBITMQ_USER", "guest"),
		RabbitMQPassword: env.GetString("RABBITMQ_PASSWORD", "guest"),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GRPCConfig.Port == "" {
		log.Fatal("USER_GRPC_PORT is required")
	}
	if c.PostgresConfig.PostgresHost == "" {
		log.Fatal("POSTGRES_HOST is required")
	}
	if c.RabbitMQ.RabbitMQHost == "" {
		log.Fatal("RABBITMQ_HOST is required")
	}

	log.Println("User Service configuration validation passed")
	return nil
}
