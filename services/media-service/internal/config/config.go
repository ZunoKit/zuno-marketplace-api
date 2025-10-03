package config

import (
	"log"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/mongo"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

// Config contains configuration for Media Service
type Config struct {
	GRPCPort     string
	MongoDB      mongo.MongoConfig
	Redis        redis.RedisConfig
	PinataConfig PinataConfig
}

type PinataConfig struct {
	BaseURL    string
	APIKey     string
	SecretKey  string
	GatewayURL string
	JWTKey     string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	log.Println("Loading Media Service configuration...")

	config := &Config{
		GRPCPort:     env.GetString("MEDIA_GRPC_PORT", ":50054"),
		MongoDB:      loadMongoConfig(),
		Redis:        loadRedisConfig(),
		PinataConfig: loadPinataConfig(),
	}

	log.Printf("Media Service config loaded - gRPC: %s",
		config.GRPCPort)

	return config
}

// loadMongoConfig loads MongoDB configuration
func loadMongoConfig() mongo.MongoConfig {
	return mongo.MongoConfig{
		MongoURI:      env.GetString("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: env.GetString("MONGO_DATABASE", "nft_marketplace"),
	}
}

// loadRedisConfig loads Redis configuration
func loadRedisConfig() redis.RedisConfig {
	return redis.RedisConfig{
		RedisHost: env.GetString("REDIS_HOST", "localhost"),
		RedisPort: env.GetInt("REDIS_PORT", 6379),
	}
}

// loadPinataConfig loads Pinata configuration
func loadPinataConfig() PinataConfig {
	return PinataConfig{
		BaseURL:    env.GetString("PINATA_BASE_URL", "https://api.pinata.cloud"),
		APIKey:     env.GetString("PINATA_API_KEY", ""),
		SecretKey:  env.GetString("PINATA_SECRET_KEY", ""),
		GatewayURL: env.GetString("PINATA_GATEWAY_URL", "https://api.pinata.cloud"),
		JWTKey:     env.GetString("PINATA_JWT_KEY", ""),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GRPCPort == "" {
		log.Fatal("GRPC_PORT is required")
	}
	if c.MongoDB.MongoURI == "" {
		log.Fatal("MONGO_URI is required")
	}
	if c.PinataConfig.APIKey == "" {
		log.Fatal("PINATA_API_KEY is required")
	}

	log.Println("Media Service configuration validation passed")
	return nil
}
