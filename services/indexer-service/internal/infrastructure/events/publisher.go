package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
)

const (
	// Collection event routing keys
	collectionEventPrefix = "collections.events.created"
	
	// Event schema versions
	eventSchemaV1 = "marketplace.events.v1"
)

type EventPublisher struct {
	amqp *messaging.RabbitMQ
}

// NewEventPublisher creates a new RabbitMQ event publisher
func NewEventPublisher(amqp *messaging.RabbitMQ) *EventPublisher {
	return &EventPublisher{
		amqp: amqp,
	}
}

// PublishCollectionEvent publishes a collection-related event
func (p *EventPublisher) PublishCollectionEvent(ctx context.Context, chainID string, event *domain.PublishableEvent) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Set default schema and version if not provided
	if event.Schema == "" {
		event.Schema = eventSchemaV1
	}
	if event.Version == "" {
		event.Version = "1.0"
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Construct routing key: collections.events.created.eip155-1 (per CREATE.md line 68)
	routingKey := fmt.Sprintf("%s.%s", collectionEventPrefix, chainID)

	// Marshal event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create message headers
	headers := map[string]interface{}{
		"event_type":    event.EventType,
		"chain_id":      event.ChainID,
		"schema":        event.Schema,
		"version":       event.Version,
		"published_at":  event.Timestamp.Unix(),
		"content_type":  "application/json",
	}

	// Publish message
	message := &messaging.Message{
		Exchange:   "collections.events",
		RoutingKey: routingKey,
		Body:       eventData,
		Headers:    headers,
		Timestamp:  event.Timestamp,
		MessageID:  event.EventID,
	}

	err = p.amqp.Publish(ctx, message.ToAMQPMessage())
	if err != nil {
		return fmt.Errorf("failed to publish collection event: %w", err)
	}

	return nil
}

// PublishCollectionCreatedEvent publishes a CollectionCreated event
func (p *EventPublisher) PublishCollectionCreatedEvent(ctx context.Context, chainID string, rawEvent *domain.RawEvent, collectionEvent *domain.CollectionCreatedEvent) error {
	// Create publishable event
	eventData := map[string]interface{}{
		"collection_address": collectionEvent.CollectionAddress,
		"creator":           collectionEvent.Creator,
		"name":              collectionEvent.Name,
		"symbol":            collectionEvent.Symbol,
		"collection_type":   collectionEvent.CollectionType,
		"max_supply":        collectionEvent.MaxSupply.String(),
		"royalty_recipient": collectionEvent.RoyaltyRecipient,
		"royalty_percentage": collectionEvent.RoyaltyPercentage,
		"block_number":      rawEvent.BlockNumber.String(),
		"block_hash":        rawEvent.BlockHash,
		"tx_hash":           rawEvent.TxHash,
		"log_index":         rawEvent.LogIndex,
		"confirmations":     rawEvent.Confirmations,
	}

	publishableEvent := &domain.PublishableEvent{
		Schema:    eventSchemaV1,
		Version:   "1.0",
		EventID:   generateEventID(chainID, rawEvent.TxHash, rawEvent.LogIndex),
		EventType: "collection_created",
		ChainID:   chainID,
		TxHash:    rawEvent.TxHash,
		Contract:  rawEvent.ContractAddress,
		Data:      eventData,
		Timestamp: time.Now(),
	}

	return p.PublishCollectionEvent(ctx, chainID, publishableEvent)
}

// PublishMintEvent publishes a mint-related event (for future use)
func (p *EventPublisher) PublishMintEvent(ctx context.Context, chainID string, event *domain.PublishableEvent) error {
	// Similar to PublishCollectionEvent but with different routing key
	routingKey := fmt.Sprintf("mints.events.minted.%s", chainID)

	// Set default values
	if event.Schema == "" {
		event.Schema = eventSchemaV1
	}
	if event.Version == "" {
		event.Version = "1.0"
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Marshal event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal mint event: %w", err)
	}

	// Create message headers
	headers := map[string]interface{}{
		"event_type":    event.EventType,
		"chain_id":      event.ChainID,
		"schema":        event.Schema,
		"version":       event.Version,
		"published_at":  event.Timestamp.Unix(),
		"content_type":  "application/json",
	}

	// Publish message
	message := &messaging.Message{
		Exchange:   "collections.events",
		RoutingKey: routingKey,
		Body:       eventData,
		Headers:    headers,
		Timestamp:  event.Timestamp,
		MessageID:  event.EventID,
	}

	err = p.amqp.Publish(ctx, message.ToAMQPMessage())
	if err != nil {
		return fmt.Errorf("failed to publish mint event: %w", err)
	}

	return nil
}

// PublishBatchEvents publishes multiple events in a batch for efficiency
func (p *EventPublisher) PublishBatchEvents(ctx context.Context, chainID string, events []*domain.PublishableEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Publish each event - in the future, this could be optimized with batch publishing
	for i, event := range events {
		err := p.PublishCollectionEvent(ctx, chainID, event)
		if err != nil {
			return fmt.Errorf("failed to publish event %d in batch: %w", i, err)
		}
	}

	return nil
}

// PublishHealthCheck publishes a health check event for monitoring
func (p *EventPublisher) PublishHealthCheck(ctx context.Context, chainID string) error {
	healthEvent := &domain.PublishableEvent{
		Schema:    eventSchemaV1,
		Version:   "1.0",
		EventID:   fmt.Sprintf("health_%s_%d", chainID, time.Now().Unix()),
		EventType: "health_check",
		ChainID:   chainID,
		TxHash:    "",
		Contract:  "",
		Data: map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
		},
		Timestamp: time.Now(),
	}

	routingKey := fmt.Sprintf("indexer.health.%s", chainID)

	// Marshal event to JSON
	eventData, err := json.Marshal(healthEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal health check event: %w", err)
	}

	// Create message
	message := &messaging.Message{
		RoutingKey: routingKey,
		Body:       eventData,
		Headers: map[string]interface{}{
			"event_type":   "health_check",
			"chain_id":     chainID,
			"published_at": time.Now().Unix(),
		},
		Timestamp: time.Now(),
		MessageID: healthEvent.EventID,
	}

	return p.amqp.Publish(ctx, message.ToAMQPMessage())
}

// generateEventID creates a unique event ID
func generateEventID(chainID, txHash string, logIndex int) string {
	return fmt.Sprintf("%s_%s_%d", chainID, txHash, logIndex)
}

// Close closes the AMQP connection
func (p *EventPublisher) Close() error {
	if p.amqp != nil {
		return p.amqp.Close()
	}
	return nil
}