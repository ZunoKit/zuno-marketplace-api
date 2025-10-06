package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/infrastructure/events"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/infrastructure/repository"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

func main() {
	cfg := config.NewConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize PostgreSQL for catalog data storage
	postgresClient, err := postgres.NewPostgres(cfg.PostgresConfig)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}

	defer postgresClient.Close()
	if err := postgresClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	redisClient, err := redis.NewRedis(cfg.RedisConfig)
	if err != nil {
		log.Fatal("Failed to load redis")
	}

	defer redisClient.Close()

	if err := redisClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}

	// Initialize RabbitMQ for event consumption and publishing
	amqpClient, err := messaging.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to create AMQP client: %v", err)
	}
	defer amqpClient.Close()

	// Initialize repositories
	collectionRepo := repository.NewCollectionRepository(postgresClient, redisClient)
	processedEventRepo := repository.NewProcessedEventRepository(postgresClient, redisClient)

	// Initialize event consumer and publisher
	consumer := events.NewEventConsumer(amqpClient, cfg.ConsumerConfig)
	publisher := events.NewEventPublisher(amqpClient)

	// Initialize catalog service
	catalogService := service.NewCatalogService(
		collectionRepo,
		processedEventRepo,
		publisher,
	)

	// Setup event handlers
	consumer.RegisterCollectionEventHandler(catalogService.HandleCollectionCreated)

	// Start consuming events in a separate goroutine
	go func() {
		log.Println("Starting catalog service event consumer...")
		if err := consumer.Start(ctx); err != nil {
			log.Fatalf("Event consumer failed: %v", err)
		}
	}()

	log.Println("Catalog service started successfully")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping catalog service...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop the consumer gracefully
	if err := consumer.Stop(shutdownCtx); err != nil {
		log.Printf("Error during consumer shutdown: %v", err)
	}

	// Close publisher
	if err := publisher.Close(); err != nil {
		log.Printf("Error closing publisher: %v", err)
	}

	log.Println("Catalog service stopped")
}
