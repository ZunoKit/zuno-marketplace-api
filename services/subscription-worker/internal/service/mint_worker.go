package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/domain"
)

// MintSubscriptionWorker handles mint events and notifications
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

	// Subscribe to mints.domain topic for mint events
	messages, err := w.messageConsumer.Subscribe(ctx, "mints.domain", "indexed.*")
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
	intents, err := w.intentRepo.GetIntentsByTxHash(ctx, event.ChainID, event.TxHash)
	if err != nil {
		return fmt.Errorf("failed to find intents: %w", err)
	}

	if len(intents) == 0 {
		// This might be an external mint not initiated through our system
		fmt.Printf("No matching intent found for tx %s, might be external mint\n", event.TxHash)
		return nil
	}

	// Use the first matching intent
	intent := intents[0]
	fmt.Printf("Found matching intent %s for tx %s\n", intent.IntentID, event.TxHash)

	// Update intent status to ready
	statusUpdate := &domain.IntentStatus{
		IntentID:        intent.IntentID,
		Status:          "ready",
		ChainID:         event.ChainID,
		TxHash:          event.TxHash,
		ContractAddress: event.Contract,
		Data: map[string]interface{}{
			"token_ids": event.TokenIDs,
		},
		UpdatedAt: time.Now(),
	}

	// Update status in cache and database
	if err := w.intentRepo.UpdateIntentStatus(ctx, statusUpdate); err != nil {
		fmt.Printf("Failed to update intent status: %v\n", err)
		// Continue to send notification
	}

	// Send WebSocket notification
	if err := w.sendMintNotification(ctx, intent, &event); err != nil {
		return fmt.Errorf("failed to send mint notification: %w", err)
	}

	fmt.Printf("Successfully processed mint for intent %s\n", intent.IntentID)
	return nil
}

// sendMintNotification sends a WebSocket notification for a completed mint
func (w *MintSubscriptionWorker) sendMintNotification(ctx context.Context, intent *domain.IntentStatus, event *MintDomainEvent) error {
	// Prepare mint status payload
	status := &MintStatus{
		IntentID:  intent.IntentID,
		Status:    "ready",
		Contract:  event.Contract,
		TokenIDs:  event.TokenIDs,
		TxHash:    event.TxHash,
		ChainID:   event.ChainID,
		Metadata:  event.Metadata,
		Timestamp: time.Now().Unix(),
	}

	// Publish to intent-specific channel
	if err := w.wsPublisher.PublishToIntent(ctx, intent.IntentID, status); err != nil {
		fmt.Printf("Failed to publish to intent channel: %v\n", err)
		// Don't fail the whole operation
	}

	fmt.Printf("Sent mint notification for intent %s\n", intent.IntentID)
	return nil
}

// HandleMintSubscription handles a new WebSocket subscription for mint status
func (w *MintSubscriptionWorker) HandleMintSubscription(ctx context.Context, intentID, subscriberID string) error {
	// Check current status from cache or database
	status, err := w.intentRepo.GetIntentStatus(ctx, intentID)
	if err != nil {
		return fmt.Errorf("intent not found: %w", err)
	}

	// Send current status immediately
	tokenIDs := []string{}
	if tokenIDsRaw, ok := status.Data["token_ids"].([]interface{}); ok {
		for _, id := range tokenIDsRaw {
			if idStr, ok := id.(string); ok {
				tokenIDs = append(tokenIDs, idStr)
			}
		}
	}

	mintStatus := &MintStatus{
		IntentID:  status.IntentID,
		Status:    status.Status,
		Contract:  status.ContractAddress,
		TokenIDs:  tokenIDs,
		TxHash:    status.TxHash,
		ChainID:   status.ChainID,
		Timestamp: time.Now().Unix(),
	}

	// Publish to intent channel
	if err := w.wsPublisher.PublishToIntent(ctx, intentID, mintStatus); err != nil {
		return fmt.Errorf("failed to send initial status: %w", err)
	}

	// Register subscription for future updates
	if err := w.wsPublisher.RegisterSubscription(ctx, intentID, subscriberID); err != nil {
		return fmt.Errorf("failed to register subscription: %w", err)
	}

	fmt.Printf("Registered mint subscription for intent %s by subscriber %s\n", intentID, subscriberID)
	return nil
}

// ProcessMintProgress processes mint progress updates
func (w *MintSubscriptionWorker) ProcessMintProgress(ctx context.Context, intentID string, progress int, message string) error {
	// Get current intent status
	status, err := w.intentRepo.GetIntentStatus(ctx, intentID)
	if err != nil {
		return fmt.Errorf("intent not found: %w", err)
	}

	// Send progress notification
	mintStatus := &MintStatus{
		IntentID:  intentID,
		Status:    "processing",
		ChainID:   status.ChainID,
		Progress:  progress,
		Message:   message,
		Timestamp: time.Now().Unix(),
	}

	return w.wsPublisher.PublishToIntent(ctx, intentID, mintStatus)
}

// Domain event structures

// MintDomainEvent represents a mint domain event from catalog service
type MintDomainEvent struct {
	Schema   string                 `json:"schema"`
	EventID  string                 `json:"event_id"`
	ChainID  string                 `json:"chain_id"`
	Contract string                 `json:"contract"`
	TokenIDs []string               `json:"token_ids"`
	Minter   string                 `json:"minter"`
	Receiver string                 `json:"receiver"`
	TxHash   string                 `json:"tx_hash"`
	Metadata map[string]interface{} `json:"metadata"`
}

// MintStatus represents the status of a mint operation
type MintStatus struct {
	IntentID  string                 `json:"intent_id"`
	Status    string                 `json:"status"`
	Contract  string                 `json:"contract,omitempty"`
	TokenIDs  []string               `json:"token_ids,omitempty"`
	TxHash    string                 `json:"tx_hash,omitempty"`
	ChainID   string                 `json:"chain_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Progress  int                    `json:"progress,omitempty"` // 0-100
	Message   string                 `json:"message,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

// WebSocketNotification represents a notification sent over WebSocket
type WebSocketNotification struct {
	Type         string      `json:"type"`
	IntentID     string      `json:"intent_id"`
	Payload      interface{} `json:"payload"`
	SubscriberID string      `json:"subscriber_id,omitempty"`
}
