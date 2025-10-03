package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/infrastructure/events"
	grpcServer "github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/infrastructure/grpc"
	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()
	cfg.Validate()

	log.Printf("Starting Wallet Service on %s", cfg.GRPCPort)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcSrv := grpc.NewServer()

	postgresDB, err := postgres.NewPostgres(cfg.Postgres)
	if err != nil {
		log.Fatalf("Failed to create postgres: %v", err)
	}
	defer postgresDB.Close()

	if err := postgresDB.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping postgres: %v", err)
	}

	redisClient, err := redis.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to create redis: %v", err)
	}
	defer redisClient.Close()

	if err := redisClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping redis: %v", err)
	}

	walletRepo := repository.NewWalletRepository(postgresDB, redisClient)
	walletService := service.NewWalletService(walletRepo)

	amqpClient, err := messaging.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to create amqp client: %v", err)
	}
	defer amqpClient.Close()

	eventPublisher := events.NewEventPublisher(amqpClient)

	walletGRPCServer := grpcServer.NewWalletGRPCServer(walletService, eventPublisher)
	wallet.RegisterWalletServiceServer(grpcSrv, walletGRPCServer)

	reflection.Register(grpcSrv)

	// Start gRPC server in a goroutine
	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCPort)
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		log.Printf("Wallet service listening on %s", cfg.GRPCPort)
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutting down Wallet Service...")
	grpcSrv.GracefulStop()
}
