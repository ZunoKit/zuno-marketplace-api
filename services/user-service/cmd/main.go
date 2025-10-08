package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/infrastructure/events"
	grpc_handler "github.com/quangdang46/NFT-Marketplace/services/user-service/internal/infrastructure/grpc"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	protoUser "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
)

func main() {
	cfg := config.NewConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	postgresClient, err := postgres.NewPostgres(cfg.PostgresConfig)
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}
	defer postgresClient.Close()
	if err := postgresClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping postgres: %v", err)
	}

	// Initialize RabbitMQ
	amqpClient, err := messaging.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to create amqp client: %v", err)
	}
	defer amqpClient.Close()

	userRepo := repository.NewUserRepository(postgresClient)
	eventPublisher := events.NewEventPublisher(amqpClient)
	userService := service.NewUserService(userRepo, eventPublisher)

	server := grpc.NewServer()
	handler := grpc_handler.NewgRPCHandler(server, userService)
	protoUser.RegisterUserServiceServer(server, handler)

	lis, err := net.Listen("tcp", cfg.GRPCConfig.Port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("User service listening on %s", cfg.GRPCConfig.Port)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
