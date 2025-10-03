package domain

import (
	"context"
	"time"
)

// IntentStatus represents the status of an intent
type IntentStatus struct {
	IntentID        string                 `json:"intent_id"`
	Status          string                 `json:"status"`
	TxHash          string                 `json:"tx_hash,omitempty"`
	ContractAddress string                 `json:"contract_address,omitempty"`
	ChainID         string                 `json:"chain_id,omitempty"`
	Data            map[string]interface{} `json:"data,omitempty"`
	UpdatedAt       time.Time              `json:"updated_at"`
	ExpiresAt       time.Time              `json:"expires_at,omitempty"`
}

// DomainEvent represents a domain event from the catalog service
type DomainEvent struct {
	Schema      string                 `json:"schema"`
	Version     string                 `json:"version"`
	EventID     string                 `json:"event_id"`
	EventType   string                 `json:"event_type"`
	AggregateID string                 `json:"aggregate_id"`
	ChainID     string                 `json:"chain_id"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   time.Time              `json:"timestamp"`
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`
	IntentID  string      `json:"intent_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

// WebSocketConnection represents a WebSocket connection
type WebSocketConnection interface {
	// Send sends a message to the client
	Send(message *WebSocketMessage) error

	// Close closes the connection
	Close() error

	// GetID returns the connection ID
	GetID() string

	// GetIntentIDs returns the intent IDs this connection is subscribed to
	GetIntentIDs() []string

	// AddIntentID adds an intent ID to the subscription list
	AddIntentID(intentID string)

	// RemoveIntentID removes an intent ID from the subscription list
	RemoveIntentID(intentID string)

	// IsActive returns whether the connection is active
	IsActive() bool
}

// Repository interfaces

type IntentRepository interface {
	// GetIntentStatus retrieves the status of an intent from Redis
	GetIntentStatus(ctx context.Context, intentID string) (*IntentStatus, error)

	// UpdateIntentStatus updates the status of an intent in Redis
	UpdateIntentStatus(ctx context.Context, status *IntentStatus) error

	// GetPendingIntentsByContract gets pending intents for a contract address
	GetPendingIntentsByContract(ctx context.Context, chainID, contractAddress string) ([]*IntentStatus, error)

	// GetIntentsByTxHash gets intents by transaction hash
	GetIntentsByTxHash(ctx context.Context, chainID, txHash string) ([]*IntentStatus, error)

	// DeleteIntent removes an intent from Redis
	DeleteIntent(ctx context.Context, intentID string) error

	// GetExpiredIntents gets all expired intents for cleanup
	GetExpiredIntents(ctx context.Context) ([]*IntentStatus, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

// Service interfaces

type WebSocketManager interface {
	// Start starts the WebSocket manager
	Start(ctx context.Context) error

	// Stop stops the WebSocket manager
	Stop(ctx context.Context) error

	// AddConnection adds a new WebSocket connection
	AddConnection(conn WebSocketConnection) error

	// RemoveConnection removes a WebSocket connection
	RemoveConnection(connID string)

	// SendToIntent sends a message to all connections subscribed to an intent
	SendToIntent(intentID string, message *WebSocketMessage) error

	// SendToConnection sends a message to a specific connection
	SendToConnection(connID string, message *WebSocketMessage) error

	// GetConnectionCount returns the number of active connections
	GetConnectionCount() int

	// Health check
	HealthCheck() error
}

type EventConsumer interface {
	// Start begins consuming events
	Start(ctx context.Context) error

	// Stop gracefully shuts down event consumption
	Stop(ctx context.Context) error

	// RegisterCollectionEventHandler registers a handler for collection domain events
	RegisterCollectionEventHandler(handler CollectionEventHandler)
}

type SubscriptionWorkerService interface {
	// HandleCollectionDomainEvent processes a collection domain event
	HandleCollectionDomainEvent(ctx context.Context, event *DomainEvent) error

	// ProcessCollectionUpserted processes a collection upserted event
	ProcessCollectionUpserted(ctx context.Context, event *DomainEvent) error

	// ResolveIntent resolves an intent and notifies subscribers
	ResolveIntent(ctx context.Context, intentID string, status *IntentStatus) error

	// SubscribeToIntent subscribes a WebSocket connection to an intent
	SubscribeToIntent(ctx context.Context, connID, intentID string) error

	// UnsubscribeFromIntent unsubscribes a WebSocket connection from an intent
	UnsubscribeFromIntent(ctx context.Context, connID, intentID string) error

	// CleanupExpiredIntents cleans up expired intents
	CleanupExpiredIntents(ctx context.Context) (int, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

// Event handler types
type CollectionEventHandler func(ctx context.Context, event *DomainEvent) error

// Helper functions for domain logic

func NewWebSocketMessage(msgType, intentID string, data interface{}) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      msgType,
		IntentID:  intentID,
		Data:      data,
		Timestamp: time.Now(),
	}
}

func NewErrorMessage(intentID, errorMsg string) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      "error",
		IntentID:  intentID,
		Error:     errorMsg,
		Timestamp: time.Now(),
	}
}

func NewStatusUpdateMessage(intentID string, status *IntentStatus) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      "status_update",
		IntentID:  intentID,
		Data:      status,
		Timestamp: time.Now(),
	}
}

func NewSuccessMessage(intentID string, data interface{}) *WebSocketMessage {
	return &WebSocketMessage{
		Type:      "success",
		IntentID:  intentID,
		Data:      data,
		Timestamp: time.Now(),
	}
}
