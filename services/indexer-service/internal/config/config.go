package config

import (
	"time"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/mongo"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
)

type Config struct {
	MongoConfig        mongo.MongoConfig
	PostgresConfig     postgres.PostgresConfig
	RabbitMQ           messaging.RabbitMQConfig
	ChainRPCs          map[string]string // chainId -> RPC URL
	FactoryContracts   map[string]string // chainId -> factory contract address
	ConfirmationBlocks map[string]int    // chainId -> number of confirmation blocks
	PollingInterval    time.Duration
}

func NewConfig() *Config {
	return &Config{
		MongoConfig: mongo.MongoConfig{
			MongoURI:      env.GetString("MONGO_URI", "mongodb://localhost:27017"),
			MongoDatabase: env.GetString("MONGO_DATABASE", "indexer"),
		},
		PostgresConfig: postgres.PostgresConfig{
			PostgresHost:     env.GetString("POSTGRES_HOST", "localhost"),
			PostgresPort:     env.GetInt("POSTGRES_PORT", 5432),
			PostgresUser:     env.GetString("POSTGRES_USER", "postgres"),
			PostgresPassword: env.GetString("POSTGRES_PASSWORD", "password"),
			PostgresDatabase: env.GetString("POSTGRES_DATABASE", "indexer_db"),
			PostgresSSLMode:  env.GetString("POSTGRES_SSL_MODE", "disable"),
		},
		RabbitMQ: messaging.RabbitMQConfig{
			RabbitMQHost:     env.GetString("RABBITMQ_HOST", "localhost"),
			RabbitMQPort:     env.GetInt("RABBITMQ_PORT", 5672),
			RabbitMQUser:     env.GetString("RABBITMQ_USER", "guest"),
			RabbitMQPassword: env.GetString("RABBITMQ_PASSWORD", "guest"),
			RabbitMQExchange: env.GetString("RABBITMQ_EXCHANGE", "nft-marketplace"),
		},
		ChainRPCs: map[string]string{
			"eip155-1":        env.GetString("ETH_MAINNET_RPC", ""), // Ethereum Mainnet
			"eip155-11155111": env.GetString("ETH_SEPOLIA_RPC", ""), // Ethereum Sepolia
			"eip155-137":      env.GetString("POLYGON_RPC", ""),     // Polygon
			"eip155-80001":    env.GetString("MUMBAI_RPC", ""),      // Polygon Mumbai
		},
		FactoryContracts: map[string]string{
			"eip155-1":        env.GetString("ETH_MAINNET_FACTORY", ""),
			"eip155-11155111": env.GetString("ETH_SEPOLIA_FACTORY", ""),
			"eip155-137":      env.GetString("POLYGON_FACTORY", ""),
			"eip155-80001":    env.GetString("MUMBAI_FACTORY", ""),
		},
		ConfirmationBlocks: map[string]int{
			"eip155-1":        env.GetInt("ETH_MAINNET_CONFIRMATIONS", 12),
			"eip155-11155111": env.GetInt("ETH_SEPOLIA_CONFIRMATIONS", 3),
			"eip155-137":      env.GetInt("POLYGON_CONFIRMATIONS", 20),
			"eip155-80001":    env.GetInt("MUMBAI_CONFIRMATIONS", 5),
		},
		PollingInterval: time.Duration(env.GetInt("POLLING_INTERVAL_SECONDS", 5)) * time.Second,
	}
}
