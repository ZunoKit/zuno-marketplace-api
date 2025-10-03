package config

import (
	"github.com/quangdang46/NFT-Marketplace/shared/env"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/mongo"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type ConsumerConfig struct {
	QueueName     string
	RoutingKeys   []string
	ConsumerTag   string
	PrefetchCount int
	AutoAck       bool
}

type Config struct {
	PostgresConfig postgres.PostgresConfig
	RabbitMQ       messaging.RabbitMQConfig
	MongoConfig    mongo.MongoConfig
	RedisConfig    redis.RedisConfig

	ConsumerConfig ConsumerConfig
}

func NewConfig() Config {
	return Config{
		PostgresConfig: loadPostgresConfig(),
		RedisConfig:    loadRedisConfig(),
		RabbitMQ:       loadRabbitMQConfig(),
		ConsumerConfig: loadConsumerConfig(),
	}
}

func loadConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		QueueName:     env.GetString("CATALOG_QUEUE_NAME", "catalog-service-queue"),
		RoutingKeys:   []string{"collections.events.created.*", "collections.events.updated.*"},
		ConsumerTag:   env.GetString("CATALOG_CONSUMER_TAG", "catalog-service-consumer"),
		PrefetchCount: env.GetInt("CATALOG_PREFETCH_COUNT", 10),
		AutoAck:       env.GetBool("CATALOG_AUTO_ACK", false),
	}
}

func loadRabbitMQConfig() messaging.RabbitMQConfig {
	return messaging.RabbitMQConfig{
		RabbitMQHost:     env.GetString("RABBITMQ_HOST", "localhost"),
		RabbitMQPort:     env.GetInt("RABBITMQ_PORT", 5671),
		RabbitMQUser:     env.GetString("RABBITMQ_USER", "guest"),
		RabbitMQPassword: env.GetString("RABBITMQ_PASSWORD", "guest"),
	}
}

func loadPostgresConfig() postgres.PostgresConfig {
	return postgres.PostgresConfig{
		PostgresHost:     env.GetString("POSTGRES_HOST", "localhost"),
		PostgresPort:     env.GetInt("POSTGRES_PORT", 5432),
		PostgresUser:     env.GetString("POSTGRES_USER", "postgres"),
		PostgresPassword: env.GetString("POSTGRES_PASSWORD", "postgres"),
		PostgresDatabase: env.GetString("POSTGRES_DATABASE", "nft_marketplace"),
		PostgresSSLMode:  env.GetString("POSTGRES_SSL_MODE", "disable"),
	}
}

// loadRedisConfig loads Redis configuration
func loadRedisConfig() redis.RedisConfig {
	return redis.RedisConfig{
		RedisHost:     env.GetString("REDIS_HOST", "localhost"),
		RedisPort:     env.GetInt("REDIS_PORT", 6379),
		RedisPassword: env.GetString("REDIS_PASSWORD", ""),
		RedisDB:       env.GetInt("REDIS_DB", 0),
	}
}
