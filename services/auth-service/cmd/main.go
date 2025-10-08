package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/infrastructure/events"
	grpc_handler "github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/infrastructure/grpc"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/middleware"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	authProto "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	protoUser "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	protoWallet "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
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

	redisClient, err := redis.NewRedis(cfg.RedisConfig)
	if err != nil {
		log.Fatalf("Failed to connect to redis: %v", err)
	}
	defer redisClient.Close()
	if err := redisClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping redis: %v", err)
	}

	authRepo := repository.NewAuthRepository(postgresClient, redisClient)

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	userConn, err := grpc.Dial(cfg.UserServiceURL, dialOptions...)
	if err != nil {
		log.Fatalf("Failed to connect to user service: %v", err)
	}
	defer userConn.Close()

	userClient := protoUser.NewUserServiceClient(userConn)

	walletConn, err := grpc.Dial(cfg.WalletServiceURL, dialOptions...)
	if err != nil {
		log.Fatalf("Failed to connect to wallet service: %v", err)
	}
	defer walletConn.Close()

	walletClient := protoWallet.NewWalletServiceClient(walletConn)

	// Initialize RabbitMQ
	amqpClient, err := messaging.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to create amqp client: %v", err)
	}
	defer amqpClient.Close()

	publisher := events.NewEventPublisher(amqpClient)

	authService := service.NewAuthService(
		authRepo,
		userClient,
		walletClient,
		publisher,
		[]byte(cfg.JWTKey),
		[]byte(cfg.RefreshKey),
		cfg.Features.EnableCollectionContext,
	)

	// Create rate limiter
	methodRateLimiter := middleware.NewMethodRateLimiter(middleware.AuthServiceRateLimitConfig())

	// Create gRPC server with interceptors
	server := grpc.NewServer(
		grpc.UnaryInterceptor(methodRateLimiter.UnaryInterceptor()),
	)

	handler := grpc_handler.NewgRPCHandler(server, authService)
	authProto.RegisterAuthServiceServer(server, handler)

	lis, err := net.Listen("tcp", cfg.GRPCConfig.Port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Auth service listening on %s", cfg.GRPCConfig.Port)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
