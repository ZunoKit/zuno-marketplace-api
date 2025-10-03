package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/config"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
)

type EventConsumer struct {
	amqp                      *messaging.RabbitMQ
	config                    config.ConsumerConfig
	collectionEventHandler    domain.CollectionEventHandler
	channel                   *amqp.Channel
	deliveries               <-chan amqp.Delivery
	done                     chan error
	consumerTag              string
	mu                       sync.RWMutex
	isRunning                bool
}

// NewEventConsumer creates a new RabbitMQ event consumer
func NewEventConsumer(amqp *messaging.RabbitMQ, config config.ConsumerConfig) *EventConsumer {
	return &EventConsumer{
		amqp:        amqp,
		config:      config,
		done:        make(chan error),
		consumerTag: config.ConsumerTag,
	}
}

// RegisterCollectionEventHandler registers a handler for collection events
func (c *EventConsumer) RegisterCollectionEventHandler(handler domain.CollectionEventHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.collectionEventHandler = handler
}

// Start begins consuming events
func (c *EventConsumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return fmt.Errorf("consumer is already running")
	}
	c.isRunning = true
	c.mu.Unlock()

	var err error

	// Get connection from the messaging client
	conn := c.amqp.GetConnection()
	if conn == nil {
		return fmt.Errorf("failed to get RabbitMQ connection")
	}

	// Create a channel
	c.channel, err = conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to create channel: %w", err)
	}

	// Set QoS (prefetch count)
	err = c.channel.Qos(c.config.PrefetchCount, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Declare the exchange (idempotent)
	err = c.channel.ExchangeDeclare(
		c.amqp.GetExchange(), // exchange name
		"topic",              // exchange type
		true,                 // durable
		false,                // auto-deleted
		false,                // internal
		false,                // no-wait
		nil,                  // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare the queue
	queue, err := c.channel.QueueDeclare(
		c.config.QueueName, // queue name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		amqp.Table{
			"x-dead-letter-exchange": c.amqp.GetExchange() + ".dlx",
		}, // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind the queue to routing keys
	for _, routingKey := range c.config.RoutingKeys {
		err = c.channel.QueueBind(
			queue.Name,           // queue name
			routingKey,           // routing key
			c.amqp.GetExchange(), // exchange
			false,                // no-wait
			nil,                  // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue to routing key %s: %w", routingKey, err)
		}
		log.Printf("Bound queue %s to routing key %s", queue.Name, routingKey)
	}

	// Start consuming
	c.deliveries, err = c.channel.Consume(
		queue.Name,      // queue
		c.consumerTag,   // consumer
		c.config.AutoAck, // auto-ack
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,             // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	log.Printf("Started consuming events from queue %s with consumer tag %s", queue.Name, c.consumerTag)

	// Process messages
	go c.processMessages(ctx)

	// Wait for done signal or context cancellation
	select {
	case <-ctx.Done():
		log.Println("Context cancelled, stopping consumer")
		return ctx.Err()
	case err := <-c.done:
		log.Printf("Consumer stopped with error: %v", err)
		return err
	}
}

// Stop gracefully shuts down event consumption
func (c *EventConsumer) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isRunning {
		return nil
	}

	log.Println("Stopping event consumer...")

	// Cancel the consumer
	if c.channel != nil {
		err := c.channel.Cancel(c.consumerTag, false)
		if err != nil {
			log.Printf("Error cancelling consumer: %v", err)
		}

		// Close the channel
		err = c.channel.Close()
		if err != nil {
			log.Printf("Error closing channel: %v", err)
		}
	}

	// Signal done
	select {
	case c.done <- nil:
	default:
	}

	c.isRunning = false
	log.Println("Event consumer stopped")

	return nil
}

// processMessages processes incoming messages
func (c *EventConsumer) processMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.done <- ctx.Err()
			return

		case delivery, ok := <-c.deliveries:
			if !ok {
				c.done <- fmt.Errorf("delivery channel closed")
				return
			}

			// Process the message
			err := c.processMessage(ctx, delivery)
			if err != nil {
				log.Printf("Error processing message: %v", err)
				// Reject the message and send to DLQ if not auto-ack
				if !c.config.AutoAck {
					delivery.Reject(false) // false = don't requeue
				}
			} else {
				// Acknowledge the message if not auto-ack
				if !c.config.AutoAck {
					delivery.Ack(false) // false = don't ack multiple
				}
			}
		}
	}
}

// processMessage processes a single message
func (c *EventConsumer) processMessage(ctx context.Context, delivery amqp.Delivery) error {
	// Add timeout to message processing
	msgCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Log message details
	log.Printf("Processing message: RoutingKey=%s, MessageID=%s, Timestamp=%v",
		delivery.RoutingKey, delivery.MessageId, delivery.Timestamp)

	// Determine event type from routing key
	eventType := c.getEventTypeFromRoutingKey(delivery.RoutingKey)
	
	switch eventType {
	case "collection_created":
		return c.processCollectionEvent(msgCtx, delivery)
	default:
		log.Printf("Unknown event type for routing key: %s", delivery.RoutingKey)
		return nil // Don't reject unknown events, just ignore them
	}
}

// processCollectionEvent processes collection-related events
func (c *EventConsumer) processCollectionEvent(ctx context.Context, delivery amqp.Delivery) error {
	// Parse the message body
	var collectionEvent domain.CollectionEvent
	err := json.Unmarshal(delivery.Body, &collectionEvent)
	if err != nil {
		return fmt.Errorf("failed to unmarshal collection event: %w", err)
	}

	// Validate event
	if err := c.validateCollectionEvent(&collectionEvent); err != nil {
		return fmt.Errorf("invalid collection event: %w", err)
	}

	// Extract chain ID from routing key if not present in event
	if collectionEvent.ChainID == "" {
		collectionEvent.ChainID = c.extractChainIDFromRoutingKey(delivery.RoutingKey)
	}

	// Add message metadata
	if collectionEvent.EventID == "" {
		collectionEvent.EventID = delivery.MessageId
	}

	// Set timestamp if not present
	if collectionEvent.Timestamp.IsZero() {
		if !delivery.Timestamp.IsZero() {
			collectionEvent.Timestamp = delivery.Timestamp
		} else {
			collectionEvent.Timestamp = time.Now()
		}
	}

	// Call the registered handler
	c.mu.RLock()
	handler := c.collectionEventHandler
	c.mu.RUnlock()

	if handler == nil {
		return fmt.Errorf("no collection event handler registered")
	}

	log.Printf("Processing collection event: EventID=%s, Type=%s, ChainID=%s, Contract=%s",
		collectionEvent.EventID, collectionEvent.EventType, collectionEvent.ChainID, collectionEvent.Contract)

	return handler(ctx, &collectionEvent)
}

// validateCollectionEvent validates the collection event structure
func (c *EventConsumer) validateCollectionEvent(event *domain.CollectionEvent) error {
	if event.EventType == "" {
		return fmt.Errorf("event_type is required")
	}

	if event.Contract == "" {
		return fmt.Errorf("contract address is required")
	}

	if event.Data == nil {
		return fmt.Errorf("event data is required")
	}

	// Validate required fields in data based on event type
	switch event.EventType {
	case "collection_created":
		requiredFields := []string{"collection_address", "creator", "name", "collection_type"}
		for _, field := range requiredFields {
			if _, exists := event.Data[field]; !exists {
				return fmt.Errorf("required field '%s' is missing from event data", field)
			}
		}
	}

	return nil
}

// getEventTypeFromRoutingKey extracts event type from routing key
func (c *EventConsumer) getEventTypeFromRoutingKey(routingKey string) string {
	// Expected format: collections.events.created.eip155-1 (per CREATE.md line 68)
	parts := strings.Split(routingKey, ".")
	if len(parts) >= 3 {
		eventType := parts[2] // "created"
		return "collection_" + eventType // return "collection_created"
	}
	return "unknown"
}

// extractChainIDFromRoutingKey extracts chain ID from routing key
func (c *EventConsumer) extractChainIDFromRoutingKey(routingKey string) string {
	// Expected format: collections.events.created.eip155-1
	parts := strings.Split(routingKey, ".")
	if len(parts) >= 4 {
		return parts[3] // "eip155-1"
	}
	return ""
}

// Health check for the consumer
func (c *EventConsumer) HealthCheck() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.isRunning {
		return fmt.Errorf("consumer is not running")
	}

	if c.channel == nil {
		return fmt.Errorf("consumer channel is nil")
	}

	if c.channel.IsClosed() {
		return fmt.Errorf("consumer channel is closed")
	}

	return nil
}

// GetConsumerInfo returns information about the consumer
func (c *EventConsumer) GetConsumerInfo() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	info := map[string]interface{}{
		"consumer_tag":    c.consumerTag,
		"queue_name":      c.config.QueueName,
		"routing_keys":    c.config.RoutingKeys,
		"prefetch_count":  c.config.PrefetchCount,
		"auto_ack":        c.config.AutoAck,
		"is_running":      c.isRunning,
	}

	if c.channel != nil {
		info["channel_open"] = !c.channel.IsClosed()
	} else {
		info["channel_open"] = false
	}

	return info
}