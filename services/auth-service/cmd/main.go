package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	// Configure TLS for client connections
	var dialOptions []grpc.DialOption
	if cfg.TLSEnabled {
		clientCreds, err := loadClientTLSCredentials("user-service")
		if err != nil {
			log.Printf("Warning: Failed to load client TLS credentials, falling back to insecure: %v", err)
			dialOptions = []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			}
		} else {
			dialOptions = []grpc.DialOption{
				grpc.WithTransportCredentials(clientCreds),
			}
		}
	} else {
		dialOptions = []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}
	}

	userConn, err := grpc.Dial(cfg.UserServiceURL, dialOptions...)
	if err != nil {
		log.Fatalf("Failed to connect to user service: %v", err)
	}
	defer userConn.Close()

	userClient := protoUser.NewUserServiceClient(userConn)

	// For wallet service, load appropriate credentials
	var walletDialOptions []grpc.DialOption
	if cfg.TLSEnabled {
		walletCreds, err := loadClientTLSCredentials("wallet-service")
		if err != nil {
			log.Printf("Warning: Failed to load wallet client TLS credentials, using same as user service")
			walletDialOptions = dialOptions
		} else {
			walletDialOptions = []grpc.DialOption{
				grpc.WithTransportCredentials(walletCreds),
			}
		}
	} else {
		walletDialOptions = dialOptions
	}

	walletConn, err := grpc.Dial(cfg.WalletServiceURL, walletDialOptions...)
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

	// Configure TLS for server
	var serverOptions []grpc.ServerOption
	if cfg.TLSEnabled {
		serverCreds, err := loadServerTLSCredentials()
		if err != nil {
			log.Printf("Warning: Failed to load server TLS credentials, running without TLS: %v", err)
			serverOptions = []grpc.ServerOption{
				grpc.UnaryInterceptor(methodRateLimiter.UnaryInterceptor()),
			}
		} else {
			serverOptions = []grpc.ServerOption{
				grpc.Creds(serverCreds),
				grpc.UnaryInterceptor(methodRateLimiter.UnaryInterceptor()),
			}
			log.Println("TLS enabled for gRPC server")
		}
	} else {
		serverOptions = []grpc.ServerOption{
			grpc.UnaryInterceptor(methodRateLimiter.UnaryInterceptor()),
		}
	}

	// Create gRPC server with interceptors
	server := grpc.NewServer(serverOptions...)

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

// loadServerTLSCredentials loads TLS credentials for the gRPC server
func loadServerTLSCredentials() (credentials.TransportCredentials, error) {
	certDir := os.Getenv("CERT_DIR")
	if certDir == "" {
		certDir = "/certs"
	}

	certFile := filepath.Join(certDir, "auth-service.crt")
	keyFile := filepath.Join(certDir, "auth-service.key")
	caFile := filepath.Join(certDir, "ca.crt")

	// Check if cert files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("server certificate not found: %s", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("server key not found: %s", keyFile)
	}

	// Load server certificate and key
	serverCert, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificates: %w", err)
	}

	// For mTLS, we would also verify client certificates
	// This requires loading the CA certificate and configuring ClientAuth
	// For now, we'll use simple TLS
	_ = caFile // Will be used for mTLS

	return serverCert, nil
}

// loadClientTLSCredentials loads TLS credentials for gRPC client connections
func loadClientTLSCredentials(serverName string) (credentials.TransportCredentials, error) {
	certDir := os.Getenv("CERT_DIR")
	if certDir == "" {
		certDir = "/certs"
	}

	caFile := filepath.Join(certDir, "ca.crt")
	clientCertFile := filepath.Join(certDir, "graphql-gateway-client.crt")
	clientKeyFile := filepath.Join(certDir, "graphql-gateway-client.key")

	// Check if CA cert exists
	if _, err := os.Stat(caFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("CA certificate not found: %s", caFile)
	}

	// For basic TLS, we only need the CA certificate
	config, err := credentials.NewClientTLSFromFile(caFile, fmt.Sprintf("%s.zuno-marketplace.local", serverName))
	if err != nil {
		return nil, fmt.Errorf("failed to load CA certificate: %w", err)
	}

	// For mTLS, we would also load client certificates
	_ = clientCertFile // Will be used for mTLS
	_ = clientKeyFile  // Will be used for mTLS

	return config, nil
}
