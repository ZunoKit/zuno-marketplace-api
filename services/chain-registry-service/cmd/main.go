package main

import (
	"context"
	"log"
	"net"

	grpc_handler "github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/infrastructure/grpc"
	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/seed"
	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/service"
	chainpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"google.golang.org/grpc"

	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/config"
	shpg "github.com/quangdang46/NFT-Marketplace/shared/postgres"
	shredis "github.com/quangdang46/NFT-Marketplace/shared/redis"
)

func main() {
	cfg := config.Load()
	cfg.Validate()

	ctx := context.Background()

	pg, err := shpg.NewPostgres(cfg.Postgres)
	if err != nil {
		log.Fatalf("failed to connect postgres: %v", err)
	}
	defer pg.Close()
	if err := pg.HealthCheck(ctx); err != nil {
		log.Fatalf("failed to ping postgres: %v", err)
	}

	// Startup seed (no S3) - best effort
	if err := seed.RunStartupSeed(pg); err != nil {
		log.Printf("seed warning: %v", err)
	}

	redis, err := shredis.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}
	defer redis.Close()
	if err := redis.HealthCheck(ctx); err != nil {
		log.Fatalf("failed to ping redis: %v", err)
	}

	repo := repository.NewRepository(pg, redis)
	svc := service.New(repo)

	server := grpc.NewServer()
	handler := grpc_handler.NewGRPCHandler(svc)
	chainpb.RegisterChainRegistryServiceServer(server, handler)

	lis, err := net.Listen("tcp", cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Printf("Chain Registry service listening on %s", cfg.GRPC.Port)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
