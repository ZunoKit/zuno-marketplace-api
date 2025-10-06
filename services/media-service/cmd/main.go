package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/config"
	grpc_handler "github.com/quangdang46/NFT-Marketplace/services/media-service/internal/infrastructure/grpc"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/infrastructure/pinning"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/mongo"
	mediaProto "github.com/quangdang46/NFT-Marketplace/shared/proto/media"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

func main() {
	// Load configuration

	cfg := config.LoadConfig()
	fmt.Println("===>JWTKey", cfg.PinataConfig.JWTKey)
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	log.Printf("Starting Media Service on %s", cfg.GRPCPort)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize MongoDB
	mongoClient, err := mongo.NewMongo(cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Close(ctx)

	if err := mongoClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	// Initialize Redis (optional, for caching)
	redisClient, err := redis.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	if err := redisClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}

	// Initialize repository
	mediaRepo := repository.NewMediaRepository(mongoClient)

	// Initialize Pinata client
	pinataClient := pinning.NewPinataClient(cfg.PinataConfig)

	// Initialize service
	mediaService := service.NewMediaService(
		mediaRepo,
		pinataClient,
	)

	// Initialize gRPC server
	server := grpc.NewServer()
	grpcHandler := grpc_handler.NewgRPCHandler(mediaService)
	mediaProto.RegisterMediaServiceServer(server, grpcHandler)

	// Start listening
	lis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down Media Service...")
		cancel()
		server.GracefulStop()
	}()

	log.Printf("Media service listening on %s", cfg.GRPCPort)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
