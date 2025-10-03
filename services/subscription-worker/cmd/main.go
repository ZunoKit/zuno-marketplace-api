package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/config"
	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/infrastructure/events"
	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/infrastructure/websocket"
	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/service"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

func main() {
	cfg := config.NewConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize Redis for intent status management
	redisClient, err := redis.NewRedis(cfg.RedisConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	if err := redisClient.HealthCheck(ctx); err != nil {
		log.Fatalf("Failed to ping Redis: %v", err)
	}

	// Initialize RabbitMQ for event consumption
	amqpClient, err := messaging.NewRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		log.Fatalf("Failed to create AMQP client: %v", err)
	}
	defer amqpClient.Close()

	// Initialize WebSocket manager
	wsManager := websocket.NewManager(cfg.WebSocketConfig)

	// Initialize event consumer
	consumer := events.NewEventConsumer(amqpClient, cfg.ConsumerConfig)

	// Initialize subscription worker service
	subscriptionService := service.NewSubscriptionWorkerService(
		redisClient,
		wsManager,
	)

	// Register event handlers
	consumer.RegisterCollectionEventHandler(subscriptionService.HandleCollectionDomainEvent)

	// Start WebSocket manager
	go func() {
		log.Println("Starting WebSocket manager...")
		if err := wsManager.Start(ctx); err != nil {
			log.Fatalf("WebSocket manager failed: %v", err)
		}
	}()

	// Start consuming events
	go func() {
		log.Println("Starting subscription worker event consumer...")
		if err := consumer.Start(ctx); err != nil {
			log.Fatalf("Event consumer failed: %v", err)
		}
	}()

	log.Println("Subscription worker service started successfully")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping subscription worker...")

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop components gracefully
	if err := consumer.Stop(shutdownCtx); err != nil {
		log.Printf("Error during consumer shutdown: %v", err)
	}

	if err := wsManager.Stop(shutdownCtx); err != nil {
		log.Printf("Error during WebSocket manager shutdown: %v", err)
	}

	log.Println("Subscription worker service stopped")
}
