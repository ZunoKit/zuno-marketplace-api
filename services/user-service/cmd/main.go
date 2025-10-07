package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/config"
	grpc_handler "github.com/quangdang46/NFT-Marketplace/services/user-service/internal/infrastructure/grpc"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	userProto "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

func main() {

	// Load configuration
	cfg := config.LoadConfig()
	cfg.Validate()

	log.Printf("Starting User Service on %s", cfg.GRPCPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	postgresClient, err := postgres.NewPostgres(cfg.Postgres)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer postgresClient.Close()

	if err := postgresClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping postgres: %v", err)
	}

	redisClient, err := redis.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}
	defer redisClient.Close()
	if err := redisClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping redis: %v", err)
	}

	userRepo := repository.NewUserRepository(postgresClient, redisClient)

	userService := service.NewUserService(userRepo)

	// Initialize gRPC handler
	server := grpc.NewServer()

	grpcHandler := grpc_handler.NewgRPCHandler(userService)
	userProto.RegisterUserServiceServer(server, grpcHandler)

	// Start listening
	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("User service listening on %s", cfg.GRPCPort)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
