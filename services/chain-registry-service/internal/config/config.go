package config

import (
	"log"
	"strconv"

	"github.com/quangdang46/NFT-Marketplace/shared/env"
	shpg "github.com/quangdang46/NFT-Marketplace/shared/postgres"
	shredis "github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type GRPCConfig struct{ Port string }

type Config struct {
	GRPC     GRPCConfig
	Postgres shpg.PostgresConfig
	Redis    shredis.RedisConfig
}

func Load() *Config {
	// Load configuration from environment variables
	postgresPort, _ := strconv.Atoi(env.GetString("POSTGRES_PORT", "5432"))
	redisPort, _ := strconv.Atoi(env.GetString("REDIS_PORT", "6379"))

	return &Config{
			GRPC: GRPCConfig{Port: env.GetString("CHAIN_REGISTRY_GRPC_PORT", ":50056")},
		Postgres: shpg.PostgresConfig{
			PostgresHost:     env.GetString("POSTGRES_HOST", "localhost"),
			PostgresPort:     postgresPort,
			PostgresUser:     env.GetString("POSTGRES_USER", "postgres"),
			PostgresPassword: env.GetString("POSTGRES_PASSWORD", "postgres"),
			PostgresDatabase: env.GetString("POSTGRES_DATABASE", "nft_marketplace"),
		},
		Redis: shredis.RedisConfig{
			RedisHost: env.GetString("REDIS_HOST", "localhost"),
			RedisPort: redisPort,
		},
	}
}

func (c *Config) Validate() {
	if c.GRPC.Port == "" {
		log.Fatal("GRPC.Port required")
	}
	if c.Postgres.PostgresHost == "" {
		log.Fatal("Postgres.Host required")
	}
}
