package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/domain"
)

// MintSubscriptionWorker handles mint event subscriptions and notifications
type MintSubscriptionWorker struct {
	intentRepo      domain.IntentRepository
	statusCache     domain.StatusCache
	wsPublisher     domain.WebSocketPublisher
	messageConsumer domain.MessageConsumer
}

// NewMintSubscriptionWorker creates a new mint subscription worker
func NewMintSubscriptionWorker(
	intentRepo domain.IntentRepository,
	statusCache domain.StatusCache,
	wsPublisher domain.WebSocketPublisher,
	messageConsumer domain.MessageConsumer,
) *MintSubscriptionWorker {
	return &MintSubscriptionWorker{
		intentRepo:      intentRepo,
		statusCache:     statusCache,
		wsPublisher:     wsPublisher,
		messageConsumer: messageConsumer,
	}
}

// Start begins consuming mint domain events
func (w *MintSubscriptionWorker) Start(ctx context.Context) error {
	fmt.Println("Starting mint subscription worker...")

	// Subscribe to mints.domain topic
	messages, err := w.messageConsumer.Subscribe(ctx, "mints.domain", "upserted.*")
	if err != nil {
		return fmt.Errorf("failed to subscribe to mints.domain: %w", err)
	}

	// Process messages
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Mint subscription worker stopped")
			return ctx.Err()
		case msg := <-messages:
			if err := w.processMintDomainEvent(ctx, msg); err != nil {
				fmt.Printf("Error processing mint domain event: %v\n", err)
				// Continue processing other messages
			}
		}
	}
}

// processMintDomainEvent processes a mint domain event
func (w *MintSubscriptionWorker) processMintDomainEvent(ctx context.Context, message []byte) error {
	// Parse the domain event
	var event MintDomainEvent
	if err := json.Unmarshal(message, &event); err != nil {
		return fmt.Errorf("failed to unmarshal domain event: %w", err)
	}

	fmt.Printf("Processing mint domain event for tx %s\n", event.TxHash)

	// Find matching intent by chain ID and transaction hash
	intent, err := w.intentRepo.FindByChainAndTxHash(ctx, event.ChainID, event.TxHash, "mint")
	if err != nil {
		if err == domain.ErrIntentNotFound {
			// This might be an external mint not initiated through our system
			fmt.Printf("No matching intent found for tx %s, might be external mint\n", event.TxHash)
			return nil
		}
		return fmt.Errorf("failed to find intent: %w", err)
	}

	fmt.Printf("Found matching intent %s for tx %s\n", intent.ID, event.TxHash)

	// Update intent status to ready
	statusUpdate := &domain.IntentStatusUpdate{
		IntentID:        intent.ID,
		Status:          "ready",
		ChainID:         event.ChainID,
		TxHash:          event.TxHash,
		ContractAddress: event.Contract,
		TokenIDs:        event.TokenIDs,
		UpdatedAt:       time.Now(),
	}

	// Update status in cache
	if err := w.statusCache.SetIntentStatus(ctx, statusUpdate); err != nil {
		fmt.Printf("Failed to update intent status in cache: %v\n", err)
		// Continue to send notification even if cache update fails
	}

	// Update intent in database
	if err := w.intentRepo.UpdateIntentStatus(ctx, intent.ID, "ready"); err != nil {
		fmt.Printf("Failed to update intent status in database: %v\n", err)
		// Continue to send notification
	}

	// Send WebSocket notification
	if err := w.sendMintNotification(ctx, intent, &event); err != nil {
		return fmt.Errorf("failed to send mint notification: %w", err)
	}

	fmt.Printf("Successfully processed mint for intent %s\n", intent.ID)
	return nil
}

// sendMintNotification sends a WebSocket notification for a completed mint
func (w *MintSubscriptionWorker) sendMintNotification(ctx context.Context, intent *domain.Intent, event *MintDomainEvent) error {
	// Prepare mint status payload
	status := &MintStatus{
		IntentID:  intent.ID,
		Status:    "ready",
		Contract:  event.Contract,
		TokenIDs:  event.TokenIDs,
		TxHash:    event.TxHash,
		ChainID:   event.ChainID,
		Metadata:  event.Metadata,
		Timestamp: time.Now().Unix(),
	}

	// Send to WebSocket subscribers
	notification := &WebSocketNotification{
		Type:         "onMintStatus",
		IntentID:     intent.ID,
		Payload:      status,
		SubscriberID: intent.CreatedBy, // Notify the creator
	}

	if err := w.wsPublisher.PublishToSubscriber(ctx, notification); err != nil {
		return fmt.Errorf("failed to publish WebSocket notification: %w", err)
	}

	// Also publish to intent-specific channel
	if err := w.wsPublisher.PublishToIntent(ctx, intent.ID, status); err != nil {
		fmt.Printf("Failed to publish to intent channel: %v\n", err)
		// Don't fail the whole operation
	}

	fmt.Printf("Sent mint notification for intent %s to user %s\n", intent.ID, intent.CreatedBy)
	return nil
}

// HandleSubscription handles a new WebSocket subscription for mint status
func (w *MintSubscriptionWorker) HandleSubscription(ctx context.Context, intentID, subscriberID string) error {
	// Check current status from cache
	status, err := w.statusCache.GetIntentStatus(ctx, intentID)
	if err != nil {
		// Status not in cache, check database
		intent, err := w.intentRepo.GetIntent(ctx, intentID)
		if err != nil {
			return fmt.Errorf("intent not found: %w", err)
		}

		// Create status from intent
		status = &domain.IntentStatusUpdate{
			IntentID: intent.ID,
			Status:   intent.Status,
			ChainID:  intent.ChainID,
			TxHash:   intent.TxHash,
		}
	}

	// Send current status immediately
	mintStatus := &MintStatus{
		IntentID:  status.IntentID,
		Status:    status.Status,
		Contract:  status.ContractAddress,
		TokenIDs:  status.TokenIDs,
		TxHash:    status.TxHash,
		ChainID:   status.ChainID,
		Timestamp: time.Now().Unix(),
	}

	notification := &WebSocketNotification{
		Type:         "onMintStatus",
		IntentID:     intentID,
		Payload:      mintStatus,
		SubscriberID: subscriberID,
	}

	if err := w.wsPublisher.PublishToSubscriber(ctx, notification); err != nil {
		return fmt.Errorf("failed to send initial status: %w", err)
	}

	// Register subscription for future updates
	if err := w.wsPublisher.RegisterSubscription(ctx, intentID, subscriberID); err != nil {
		return fmt.Errorf("failed to register subscription: %w", err)
	}

	fmt.Printf("Registered mint subscription for intent %s by subscriber %s\n", intentID, subscriberID)
	return nil
}

// GetMintStatus retrieves the current status of a mint intent
func (w *MintSubscriptionWorker) GetMintStatus(ctx context.Context, intentID string) (*MintStatus, error) {
	// Try cache first
	status, err := w.statusCache.GetIntentStatus(ctx, intentID)
	if err == nil {
		return &MintStatus{
			IntentID:  status.IntentID,
			Status:    status.Status,
			Contract:  status.ContractAddress,
			TokenIDs:  status.TokenIDs,
			TxHash:    status.TxHash,
			ChainID:   status.ChainID,
			Timestamp: status.UpdatedAt.Unix(),
		}, nil
	}

	// Fallback to database
	intent, err := w.intentRepo.GetIntent(ctx, intentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get intent: %w", err)
	}

	return &MintStatus{
		IntentID:  intent.ID,
		Status:    intent.Status,
		Contract:  intent.ContractAddress,
		TxHash:    intent.TxHash,
		ChainID:   intent.ChainID,
		Timestamp: intent.UpdatedAt.Unix(),
	}, nil
}

// Domain event structures

// MintDomainEvent represents a mint domain event from catalog service
type MintDomainEvent struct {
	Schema    string                 `json:"schema"`
	EventType string                 `json:"event_type"`
	ChainID   string                 `json:"chain_id"`
	Contract  string                 `json:"contract"`
	TokenIDs  []string               `json:"token_ids"`
	Owner     string                 `json:"owner"`
	TxHash    string                 `json:"tx_hash"`
	Standard  string                 `json:"standard"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// MintStatus represents the status of a mint operation
type MintStatus struct {
	IntentID  string                 `json:"intent_id"`
	Status    string                 `json:"status"`
	Contract  string                 `json:"contract,omitempty"`
	TokenIDs  []string               `json:"token_ids,omitempty"`
	TxHash    string                 `json:"tx_hash,omitempty"`
	ChainID   string                 `json:"chain_id,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// WebSocketNotification represents a WebSocket notification
type WebSocketNotification struct {
	Type         string      `json:"type"`
	IntentID     string      `json:"intent_id"`
	Payload      interface{} `json:"payload"`
	SubscriberID string      `json:"subscriber_id"`
}
