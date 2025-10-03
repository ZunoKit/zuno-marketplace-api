package config

import (
	"time"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type ConsumerConfig struct {
	QueueName     string
	RoutingKeys   []string
	ConsumerTag   string
	PrefetchCount int
	AutoAck       bool
}

type WebSocketConfig struct {
	Host              string
	Port              string
	ConnectionTimeout time.Duration
	PingInterval      time.Duration
	MaxConnections    int
	MaxMessageSize    int64
	EnableCompression bool
}

type Config struct {
	RedisConfig     redis.RedisConfig
	RabbitMQ        messaging.RabbitMQConfig
	ConsumerConfig  ConsumerConfig
	WebSocketConfig WebSocketConfig
}

func NewConfig() *Config {
	return &Config{
		RedisConfig: redis.RedisConfig{
			RedisHost:     env.GetString("REDIS_HOST", "localhost"),
			RedisPort:     env.GetInt("REDIS_PORT", 6379),
			RedisPassword: env.GetString("REDIS_PASSWORD", ""),
			RedisDB:       env.GetInt("REDIS_DB", 0),
		},
		RabbitMQ: messaging.RabbitMQConfig{
			RabbitMQHost:     env.GetString("RABBITMQ_HOST", "localhost"),
			RabbitMQPort:     env.GetInt("RABBITMQ_PORT", 5672),
			RabbitMQUser:     env.GetString("RABBITMQ_USER", "guest"),
			RabbitMQPassword: env.GetString("RABBITMQ_PASSWORD", "guest"),
			RabbitMQExchange: env.GetString("RABBITMQ_EXCHANGE", "nft-marketplace"),
		},
		ConsumerConfig: ConsumerConfig{
			QueueName: env.GetString("SUBSCRIPTION_QUEUE_NAME", "subscription.collections.domain"),
			RoutingKeys: []string{
				"collections.domain.upserted.eip155-1",        // Ethereum Mainnet
				"collections.domain.upserted.eip155-11155111", // Ethereum Sepolia
				"collections.domain.upserted.eip155-137",      // Polygon
				"collections.domain.upserted.eip155-80001",    // Polygon Mumbai
				"collections.domain.upserted.*",               // Catch-all for new chains
			},
			ConsumerTag:   env.GetString("SUBSCRIPTION_CONSUMER_TAG", "subscription-worker"),
			PrefetchCount: env.GetInt("SUBSCRIPTION_PREFETCH_COUNT", 10),
			AutoAck:       env.GetBool("SUBSCRIPTION_AUTO_ACK", false),
		},
		WebSocketConfig: WebSocketConfig{
			Host:              env.GetString("WEBSOCKET_HOST", "0.0.0.0"),
			Port:              env.GetString("WEBSOCKET_PORT", "8081"),
			ConnectionTimeout: time.Duration(env.GetInt("WEBSOCKET_CONNECTION_TIMEOUT_SECONDS", 30)) * time.Second,
			PingInterval:      time.Duration(env.GetInt("WEBSOCKET_PING_INTERVAL_SECONDS", 30)) * time.Second,
			MaxConnections:    env.GetInt("WEBSOCKET_MAX_CONNECTIONS", 1000),
			MaxMessageSize:    int64(env.GetInt("WEBSOCKET_MAX_MESSAGE_SIZE", 1024*1024)), // 1MB
			EnableCompression: env.GetBool("WEBSOCKET_ENABLE_COMPRESSION", true),
		},
	}
}
