package config

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

// GRPCConfig holds gRPC server configuration
type GRPCConfig struct {
	Port string
}

// Config contains configuration for Auth Service
type Config struct {
	GRPCConfig       GRPCConfig
	JWTKey           string
	RefreshKey       string
	UserServiceURL   string
	WalletServiceURL string
	PostgresConfig   postgres.PostgresConfig
	RedisConfig      redis.RedisConfig
	RabbitMQ         messaging.RabbitMQConfig
	Features         Features
}

// NewConfig creates and loads configuration from environment variables
func NewConfig() *Config {
	log.Println("Loading Auth Service configuration...")

	config := &Config{
		GRPCConfig: GRPCConfig{
			Port: env.GetString("AUTH_GRPC_PORT", ":50051"),
		},
		JWTKey:           env.GetString("JWT_SECRET", "default-jwt-secret-for-development"),
		RefreshKey:       env.GetString("REFRESH_SECRET", "default-refresh-secret-for-development"),
		UserServiceURL:   env.GetString("USER_SERVICE_URL", "user-service:50052"),
		WalletServiceURL: env.GetString("WALLET_SERVICE_URL", "wallet-service:50053"),
		PostgresConfig:   loadPostgresConfig(),
		RedisConfig:      loadRedisConfig(),
		RabbitMQ:         loadRabbitMQConfig(),
		Features:         loadFeatures(),
	}

	return config
}

// LoadConfig is an alias for NewConfig for consistency
func LoadConfig() *Config {
	return NewConfig()
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

// loadRedisConfig loads Redis configuration
func loadRedisConfig() redis.RedisConfig {
	return redis.RedisConfig{
		RedisHost: env.GetString("REDIS_HOST", "localhost"),
		RedisPort: env.GetInt("REDIS_PORT", 6379),
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

// Features holds feature flags for gradual rollout
type Features struct {
	EnableCollectionContext bool
}

func loadFeatures() Features {
	return Features{
		EnableCollectionContext: env.GetBool("ENABLE_COLLECTION_CONTEXT", false),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GRPCConfig.Port == "" {
		log.Fatal("GRPC_PORT is required")
	}
	if c.JWTKey == "" {
		log.Fatal("JWT_SECRET is required")
	}
	if c.PostgresConfig.PostgresHost == "" {
		log.Fatal("POSTGRES_HOST is required")
	}
	if c.RedisConfig.RedisHost == "" {
		log.Fatal("REDIS_HOST is required")
	}
	if c.RabbitMQ.RabbitMQHost == "" {
		log.Fatal("AMQP_HOST is required")
	}

	log.Println("Auth Service configuration validation passed")
	return nil
}
