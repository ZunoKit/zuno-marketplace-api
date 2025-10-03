package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
)

const (
	// Domain event routing keys (per CREATE.md line 74)
	collectionDomainPrefix = "collections.domain.upserted"
	
	// Event schema versions
	domainSchemaV1 = "marketplace.domain.v1"
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

// PublishDomainEvent publishes a domain event
func (p *EventPublisher) PublishDomainEvent(ctx context.Context, event *domain.DomainEvent) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	// Set default schema and version if not provided
	if event.Schema == "" {
		event.Schema = domainSchemaV1
	}
	if event.Version == "" {
		event.Version = "1.0"
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Construct routing key based on event type, chain ID and contract (per CREATE.md line 74)
	var routingKey string
	contractAddr := ""
	if event.Data != nil {
		if addr, ok := event.Data["contract_address"].(string); ok {
			contractAddr = addr
		}
	}
	
	switch event.EventType {
	case "collection_upserted", "collection_created":
		if contractAddr != "" {
			routingKey = fmt.Sprintf("%s.%s.%s", collectionDomainPrefix, event.ChainID, contractAddr)
		} else {
			routingKey = fmt.Sprintf("%s.%s", collectionDomainPrefix, event.ChainID)
		}
	default:
		routingKey = fmt.Sprintf("collections.domain.%s.%s", event.EventType, event.ChainID)
	}

	// Marshal event to JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal domain event: %w", err)
	}

	// Create message headers
	headers := map[string]interface{}{
		"event_type":    event.EventType,
		"aggregate_id":  event.AggregateID,
		"chain_id":      event.ChainID,
		"schema":        event.Schema,
		"version":       event.Version,
		"published_at":  event.Timestamp.Unix(),
		"content_type":  "application/json",
	}

	// Publish message
	message := &messaging.Message{
		RoutingKey: routingKey,
		Body:       eventData,
		Headers:    headers,
		Timestamp:  event.Timestamp,
		MessageID:  event.EventID,
	}

	err = p.amqp.Publish(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to publish domain event: %w", err)
	}

	return nil
}

// PublishCollectionUpserted publishes a collection upserted event
func (p *EventPublisher) PublishCollectionUpserted(ctx context.Context, collection *domain.Collection) error {
	if collection == nil {
		return fmt.Errorf("collection cannot be nil")
	}

	// Create domain event data
	eventData := map[string]interface{}{
		"id":                  collection.ID,
		"slug":                collection.Slug,
		"name":                collection.Name,
		"description":         collection.Description,
		"chain_id":            collection.ChainID,
		"contract_address":    collection.ContractAddress,
		"creator":             collection.Creator,
		"owner":               collection.Owner,
		"collection_type":     collection.CollectionType,
		"max_supply":          collection.MaxSupply.String(),
		"total_supply":        collection.TotalSupply.String(),
		"royalty_recipient":   collection.RoyaltyRecipient,
		"royalty_percentage":  collection.RoyaltyPercentage,
		"is_verified":         collection.IsVerified,
		"is_explicit":         collection.IsExplicit,
		"is_featured":         collection.IsFeatured,
		"image_url":           collection.ImageURL,
		"banner_url":          collection.BannerURL,
		"external_url":        collection.ExternalURL,
		"discord_url":         collection.DiscordURL,
		"twitter_url":         collection.TwitterURL,
		"instagram_url":       collection.InstagramURL,
		"telegram_url":        collection.TelegramURL,
		"floor_price":         collection.FloorPrice.String(),
		"volume_traded":       collection.VolumeTraded.String(),
		"created_at":          collection.CreatedAt,
		"updated_at":          collection.UpdatedAt,
	}

	// Create domain event
	domainEvent := &domain.DomainEvent{
		Schema:      domainSchemaV1,
		Version:     "1.0",
		EventID:     generateDomainEventID("collection_upserted", collection.ID),
		EventType:   "collection_upserted",
		AggregateID: collection.ID,
		ChainID:     collection.ChainID,
		Data:        eventData,
		Timestamp:   time.Now(),
	}

	return p.PublishDomainEvent(ctx, domainEvent)
}

// PublishCollectionCreated publishes a collection created domain event
func (p *EventPublisher) PublishCollectionCreated(ctx context.Context, collection *domain.Collection) error {
	if collection == nil {
		return fmt.Errorf("collection cannot be nil")
	}

	// Create domain event data (subset for creation event)
	eventData := map[string]interface{}{
		"id":                 collection.ID,
		"slug":               collection.Slug,
		"name":               collection.Name,
		"chain_id":           collection.ChainID,
		"contract_address":   collection.ContractAddress,
		"creator":            collection.Creator,
		"collection_type":    collection.CollectionType,
		"max_supply":         collection.MaxSupply.String(),
		"royalty_recipient":  collection.RoyaltyRecipient,
		"royalty_percentage": collection.RoyaltyPercentage,
		"created_at":         collection.CreatedAt,
	}

	// Create domain event
	domainEvent := &domain.DomainEvent{
		Schema:      domainSchemaV1,
		Version:     "1.0",
		EventID:     generateDomainEventID("collection_created", collection.ID),
		EventType:   "collection_created",
		AggregateID: collection.ID,
		ChainID:     collection.ChainID,
		Data:        eventData,
		Timestamp:   time.Now(),
	}

	return p.PublishDomainEvent(ctx, domainEvent)
}

// PublishCollectionUpdated publishes a collection updated domain event
func (p *EventPublisher) PublishCollectionUpdated(ctx context.Context, collection *domain.Collection) error {
	if collection == nil {
		return fmt.Errorf("collection cannot be nil")
	}

	// Create domain event data
	eventData := map[string]interface{}{
		"id":               collection.ID,
		"slug":             collection.Slug,
		"name":             collection.Name,
		"description":      collection.Description,
		"owner":            collection.Owner,
		"is_verified":      collection.IsVerified,
		"is_explicit":      collection.IsExplicit,
		"is_featured":      collection.IsFeatured,
		"image_url":        collection.ImageURL,
		"banner_url":       collection.BannerURL,
		"external_url":     collection.ExternalURL,
		"discord_url":      collection.DiscordURL,
		"twitter_url":      collection.TwitterURL,
		"instagram_url":    collection.InstagramURL,
		"telegram_url":     collection.TelegramURL,
		"floor_price":      collection.FloorPrice.String(),
		"volume_traded":    collection.VolumeTraded.String(),
		"updated_at":       collection.UpdatedAt,
	}

	// Create domain event
	domainEvent := &domain.DomainEvent{
		Schema:      domainSchemaV1,
		Version:     "1.0",
		EventID:     generateDomainEventID("collection_updated", collection.ID),
		EventType:   "collection_updated",
		AggregateID: collection.ID,
		ChainID:     collection.ChainID,
		Data:        eventData,
		Timestamp:   time.Now(),
	}

	return p.PublishDomainEvent(ctx, domainEvent)
}

// PublishBatchDomainEvents publishes multiple domain events in a batch
func (p *EventPublisher) PublishBatchDomainEvents(ctx context.Context, events []*domain.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	// Publish each event - in the future, this could be optimized with batch publishing
	for i, event := range events {
		err := p.PublishDomainEvent(ctx, event)
		if err != nil {
			return fmt.Errorf("failed to publish domain event %d in batch: %w", i, err)
		}
	}

	return nil
}

// PublishHealthCheck publishes a health check event for monitoring
func (p *EventPublisher) PublishHealthCheck(ctx context.Context, serviceID string) error {
	healthEvent := &domain.DomainEvent{
		Schema:      domainSchemaV1,
		Version:     "1.0",
		EventID:     fmt.Sprintf("health_%s_%d", serviceID, time.Now().Unix()),
		EventType:   "health_check",
		AggregateID: serviceID,
		ChainID:     "all",
		Data: map[string]interface{}{
			"service_id": serviceID,
			"status":     "healthy",
			"timestamp":  time.Now().Unix(),
		},
		Timestamp: time.Now(),
	}

	routingKey := fmt.Sprintf("catalog.health.%s", serviceID)

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
			"service_id":   serviceID,
			"published_at": time.Now().Unix(),
		},
		Timestamp: time.Now(),
		MessageID: healthEvent.EventID,
	}

	return p.amqp.Publish(ctx, message)
}

// generateDomainEventID creates a unique event ID for domain events
func generateDomainEventID(eventType, aggregateID string) string {
	return fmt.Sprintf("%s_%s_%d", eventType, aggregateID, time.Now().UnixNano())
}

// Close closes the AMQP connection
func (p *EventPublisher) Close() error {
	if p.amqp != nil {
		return p.amqp.Close()
	}
	return nil
}