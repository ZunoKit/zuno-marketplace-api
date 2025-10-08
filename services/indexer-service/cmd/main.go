package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/infrastructure/blockchain"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/infrastructure/events"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/mongo"
	"github.com/quangdang46/NFT-Marketplace/shared/monitoring"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
)

func main() {
	cfg := config.NewConfig()

	// Initialize Sentry
	if err := monitoring.InitSentry(&monitoring.SentryConfig{
		DSN:              os.Getenv("SENTRY_DSN"),
		Environment:      os.Getenv("ENVIRONMENT"),
		Release:          os.Getenv("RELEASE_VERSION"),
		ServiceName:      "indexer-service",
		SampleRate:       1.0,
		TracesSampleRate: 0.1,
		Debug:            os.Getenv("SENTRY_DEBUG") == "true",
	}); err != nil {
		log.Printf("Failed to initialize Sentry: %v", err)
	}
	defer sentry.Flush(2 * time.Second)

	// Setup panic recovery with Sentry
	defer monitoring.RecoverWithSentry()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize MongoDB for raw events storage
	mongoClient, err := mongo.NewMongo(cfg.MongoConfig)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Close(ctx)
	if err := mongoClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	// Initialize PostgreSQL for checkpoint management
	postgresClient, err := postgres.NewPostgres(cfg.PostgresConfig)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer postgresClient.Close()
	if err := postgresClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	// Initialize RabbitMQ for event publishing
	amqpClient, err := messaging.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to create AMQP client: %v", err)
	}
	defer amqpClient.Close()

	// Initialize repositories
	eventRepo := repository.NewEventRepository(mongoClient)
	checkpointRepo := repository.NewCheckpointRepository(postgresClient)

	// Initialize event publisher
	publisher := events.NewEventPublisher(amqpClient)

	// Initialize blockchain clients
	blockchainClients := make(map[string]*blockchain.Client)
	for chainID, rpcURL := range cfg.ChainRPCs {
		client, err := blockchain.NewClient(chainID, rpcURL, cfg.ConfirmationBlocks)
		if err != nil {
			log.Fatalf("Failed to create blockchain client for chain %s: %v", chainID, err)
		}
		blockchainClients[chainID] = client
	}

	// Initialize indexer service
	indexerService := service.NewIndexerService(
		eventRepo,
		checkpointRepo,
		publisher,
		blockchainClients,
		cfg.FactoryContracts,
		cfg.PollingInterval,
	)

	// Start indexing in a separate goroutine
	go func() {
		log.Println("Starting blockchain indexer...")
		if err := indexerService.Start(ctx); err != nil {
			log.Fatalf("Indexer service failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping indexer...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop the indexer service gracefully
	if err := indexerService.Stop(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Indexer service stopped")
}
