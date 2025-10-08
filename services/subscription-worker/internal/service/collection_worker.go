package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/domain"
)

// CollectionSubscriptionWorker handles collection creation events and notifications
type CollectionSubscriptionWorker struct {
	intentRepo      domain.IntentRepository
	statusCache     domain.StatusCache
	wsPublisher     domain.WebSocketPublisher
	messageConsumer domain.MessageConsumer
}

// NewCollectionSubscriptionWorker creates a new collection subscription worker
func NewCollectionSubscriptionWorker(
	intentRepo domain.IntentRepository,
	statusCache domain.StatusCache,
	wsPublisher domain.WebSocketPublisher,
	messageConsumer domain.MessageConsumer,
) *CollectionSubscriptionWorker {
	return &CollectionSubscriptionWorker{
		intentRepo:      intentRepo,
		statusCache:     statusCache,
		wsPublisher:     wsPublisher,
		messageConsumer: messageConsumer,
	}
}

// Start begins consuming collection domain events
func (w *CollectionSubscriptionWorker) Start(ctx context.Context) error {
	fmt.Println("Starting collection subscription worker...")

	// Subscribe to collection.domain topic for collection events
	messages, err := w.messageConsumer.Subscribe(ctx, "collections.domain", "indexed.*")
	if err != nil {
		return fmt.Errorf("failed to subscribe to collections.domain: %w", err)
	}

	// Process messages
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Collection subscription worker stopped")
			return ctx.Err()
		case msg := <-messages:
			if err := w.processCollectionDomainEvent(ctx, msg); err != nil {
				fmt.Printf("Error processing collection domain event: %v\n", err)
				// Continue processing other messages
			}
		}
	}
}

// processCollectionDomainEvent processes a collection domain event
func (w *CollectionSubscriptionWorker) processCollectionDomainEvent(ctx context.Context, message []byte) error {
	// Parse the domain event
	var event CollectionDomainEvent
	if err := json.Unmarshal(message, &event); err != nil {
		return fmt.Errorf("failed to unmarshal domain event: %w", err)
	}

	fmt.Printf("Processing collection domain event for %s on chain %s\n", event.CollectionAddress, event.ChainID)

	// Find matching intent by chain ID and collection address
	intents, err := w.intentRepo.GetPendingIntentsByContract(ctx, event.ChainID, event.CollectionAddress)
	if err != nil {
		return fmt.Errorf("failed to find intents: %w", err)
	}

	if len(intents) == 0 {
		// This might be an external collection creation
		fmt.Printf("No matching intent found for collection %s, might be external\n", event.CollectionAddress)
		return nil
	}

	// Use the first matching intent
	intent := intents[0]
	fmt.Printf("Found matching intent %s for collection %s\n", intent.IntentID, event.CollectionAddress)

	// Update intent status to ready
	statusUpdate := &domain.IntentStatus{
		IntentID:        intent.IntentID,
		Status:          "ready",
		ChainID:         event.ChainID,
		TxHash:          event.TxHash,
		ContractAddress: event.CollectionAddress,
		UpdatedAt:       time.Now(),
	}

	// Update status in cache and database
	if err := w.intentRepo.UpdateIntentStatus(ctx, statusUpdate); err != nil {
		fmt.Printf("Failed to update intent status: %v\n", err)
		// Continue to send notification
	}

	// Send WebSocket notification
	if err := w.sendCollectionNotification(ctx, intent, &event); err != nil {
		return fmt.Errorf("failed to send collection notification: %w", err)
	}

	// Publish collection ready event for marketplace listing
	if err := w.publishCollectionReady(ctx, &event); err != nil {
		fmt.Printf("Failed to publish collection ready event: %v\n", err)
		// Don't fail the whole operation
	}

	fmt.Printf("Successfully processed collection creation for intent %s\n", intent.IntentID)
	return nil
}

// sendCollectionNotification sends a WebSocket notification for a completed collection
func (w *CollectionSubscriptionWorker) sendCollectionNotification(ctx context.Context, intent *domain.IntentStatus, event *CollectionDomainEvent) error {
	// Prepare collection status payload
	status := &CollectionStatus{
		IntentID:        intent.IntentID,
		Status:          "ready",
		ContractAddress: event.CollectionAddress,
		TxHash:          event.TxHash,
		ChainID:         event.ChainID,
		Name:            event.Name,
		Symbol:          event.Symbol,
		Creator:         event.Creator,
		TotalSupply:     event.TotalSupply,
		CollectionType:  event.CollectionType,
		Timestamp:       time.Now().Unix(),
	}

	// Also publish to intent-specific channel
	if err := w.wsPublisher.PublishToIntent(ctx, intent.IntentID, status); err != nil {
		fmt.Printf("Failed to publish to intent channel: %v\n", err)
		// Don't fail the whole operation
	}

	fmt.Printf("Sent collection notification for intent %s\n", intent.IntentID)
	return nil
}

// publishCollectionReady publishes a collection ready event for marketplace listing
func (w *CollectionSubscriptionWorker) publishCollectionReady(ctx context.Context, event *CollectionDomainEvent) error {
	readyEvent := map[string]interface{}{
		"schema":             "v1",
		"event_type":         "collection.ready",
		"chain_id":           event.ChainID,
		"collection_address": event.CollectionAddress,
		"name":               event.Name,
		"symbol":             event.Symbol,
		"creator":            event.Creator,
		"type":               event.CollectionType,
		"metadata": map[string]interface{}{
			"logo_uri":   event.LogoURI,
			"banner_uri": event.BannerURI,
		},
		"timestamp": time.Now().Unix(),
	}

	// Publish to marketplace topic
	return w.messageConsumer.Publish(ctx, "marketplace.collections", "ready", readyEvent)
}

// HandleCollectionSubscription handles a new WebSocket subscription for collection status
func (w *CollectionSubscriptionWorker) HandleCollectionSubscription(ctx context.Context, intentID, subscriberID string) error {
	// Check current status from cache or database
	status, err := w.intentRepo.GetIntentStatus(ctx, intentID)
	if err != nil {
		return fmt.Errorf("intent not found: %w", err)
	}

	// Send current status immediately
	collectionStatus := &CollectionStatus{
		IntentID:        status.IntentID,
		Status:          status.Status,
		ContractAddress: status.ContractAddress,
		TxHash:          status.TxHash,
		ChainID:         status.ChainID,
		Timestamp:       time.Now().Unix(),
	}

	// Publish to intent channel
	if err := w.wsPublisher.PublishToIntent(ctx, intentID, collectionStatus); err != nil {
		return fmt.Errorf("failed to send initial status: %w", err)
	}

	// Register subscription for future updates
	if err := w.wsPublisher.RegisterSubscription(ctx, intentID, subscriberID); err != nil {
		return fmt.Errorf("failed to register subscription: %w", err)
	}

	fmt.Printf("Registered collection subscription for intent %s by subscriber %s\n", intentID, subscriberID)
	return nil
}

// ProcessCollectionIndexedEvent processes when a collection has been fully indexed
func (w *CollectionSubscriptionWorker) ProcessCollectionIndexedEvent(ctx context.Context, event *CollectionIndexedEvent) error {
	fmt.Printf("Processing collection indexed event for %s\n", event.CollectionAddress)

	// Find the related intents
	intents, err := w.intentRepo.GetPendingIntentsByContract(ctx, event.ChainID, event.CollectionAddress)
	if err != nil {
		return fmt.Errorf("failed to find intents: %w", err)
	}

	if len(intents) == 0 {
		// External collection, skip
		fmt.Printf("No matching intent found for collection %s\n", event.CollectionAddress)
		return nil
	}

	// Use the first matching intent
	intent := intents[0]

	// Update intent status to "indexed"
	statusUpdate := &domain.IntentStatus{
		IntentID:        intent.IntentID,
		Status:          "indexed",
		ChainID:         event.ChainID,
		ContractAddress: event.CollectionAddress,
		UpdatedAt:       time.Now(),
	}

	if err := w.intentRepo.UpdateIntentStatus(ctx, statusUpdate); err != nil {
		fmt.Printf("Failed to update intent status: %v\n", err)
	}

	// Send notification about indexing completion
	status := &CollectionStatus{
		IntentID:        intent.IntentID,
		Status:          "indexed",
		ContractAddress: event.CollectionAddress,
		ChainID:         event.ChainID,
		Progress:        100, // Indexing complete
		Message:         "Collection has been fully indexed and is now searchable",
		Timestamp:       time.Now().Unix(),
	}

	return w.wsPublisher.PublishToIntent(ctx, intent.IntentID, status)
}

// Domain event structures

// CollectionDomainEvent represents a collection domain event from catalog service
type CollectionDomainEvent struct {
	Schema            string `json:"schema"`
	EventType         string `json:"event_type"`
	ChainID           string `json:"chain_id"`
	CollectionAddress string `json:"collection_address"`
	Name              string `json:"name"`
	Symbol            string `json:"symbol"`
	Creator           string `json:"creator"`
	TotalSupply       int    `json:"total_supply"`
	CollectionType    string `json:"collection_type"` // ERC721 or ERC1155
	TxHash            string `json:"tx_hash"`
	LogoURI           string `json:"logo_uri"`
	BannerURI         string `json:"banner_uri"`
}

// CollectionIndexedEvent represents when a collection has been indexed
type CollectionIndexedEvent struct {
	ChainID           string `json:"chain_id"`
	CollectionAddress string `json:"collection_address"`
	TokenCount        int    `json:"token_count"`
	LastIndexedBlock  int64  `json:"last_indexed_block"`
}

// CollectionStatus represents the status of a collection creation
type CollectionStatus struct {
	IntentID        string `json:"intent_id"`
	Status          string `json:"status"`
	ContractAddress string `json:"contract_address,omitempty"`
	TxHash          string `json:"tx_hash,omitempty"`
	ChainID         string `json:"chain_id,omitempty"`
	Name            string `json:"name,omitempty"`
	Symbol          string `json:"symbol,omitempty"`
	Creator         string `json:"creator,omitempty"`
	TotalSupply     int    `json:"total_supply,omitempty"`
	CollectionType  string `json:"collection_type,omitempty"`
	Progress        int    `json:"progress,omitempty"` // 0-100
	Message         string `json:"message,omitempty"`
	Error           string `json:"error,omitempty"`
	Timestamp       int64  `json:"timestamp"`
}

// WebSocketNotification is defined in mint_worker.go to avoid duplication
