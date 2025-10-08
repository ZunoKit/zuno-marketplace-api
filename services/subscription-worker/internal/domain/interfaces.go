package domain

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrIntentNotFound = errors.New("intent not found")
	ErrInvalidStatus  = errors.New("invalid status")
)

// Intent represents a transaction intent
type Intent struct {
	ID              string
	Kind            string // "mint" or "collection"
	Status          string
	ChainID         string
	TxHash          string
	ContractAddress string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IntentRepository defines the interface for intent storage
type IntentRepository interface {
	// GetIntent retrieves an intent by ID
	GetIntent(ctx context.Context, intentID string) (*Intent, error)

	// FindByChainAndTxHash finds an intent by chain ID and transaction hash
	FindByChainAndTxHash(ctx context.Context, chainID, txHash, kind string) (*Intent, error)

	// FindByChainAndContract finds an intent by chain ID and contract address
	FindByChainAndContract(ctx context.Context, chainID, contractAddress, kind string) (*Intent, error)

	// UpdateIntentStatus updates the status of an intent
	UpdateIntentStatus(ctx context.Context, intentID, status string) error
}

// IntentStatusUpdate represents an intent status update
type IntentStatusUpdate struct {
	IntentID        string
	Status          string
	ChainID         string
	TxHash          string
	ContractAddress string
	TokenIDs        []string
	UpdatedAt       time.Time
}

// StatusCache defines the interface for status caching
type StatusCache interface {
	// SetIntentStatus sets the status of an intent in cache
	SetIntentStatus(ctx context.Context, status *IntentStatusUpdate) error

	// GetIntentStatus gets the status of an intent from cache
	GetIntentStatus(ctx context.Context, intentID string) (*IntentStatusUpdate, error)
}

// WebSocketPublisher defines the interface for WebSocket publishing
type WebSocketPublisher interface {
	// PublishToSubscriber publishes a message to a specific subscriber
	PublishToSubscriber(ctx context.Context, notification interface{}) error

	// PublishToIntent publishes a message to all subscribers of an intent
	PublishToIntent(ctx context.Context, intentID string, message interface{}) error

	// RegisterSubscription registers a subscription for an intent
	RegisterSubscription(ctx context.Context, intentID, subscriberID string) error

	// UnregisterSubscription removes a subscription
	UnregisterSubscription(ctx context.Context, intentID, subscriberID string) error
}

// MessageConsumer defines the interface for message queue consumption
type MessageConsumer interface {
	// Subscribe subscribes to a topic with routing key pattern
	Subscribe(ctx context.Context, topic, routingKey string) (<-chan []byte, error)

	// Publish publishes a message to a topic
	Publish(ctx context.Context, topic, routingKey string, message interface{}) error

	// Acknowledge acknowledges a message
	Acknowledge(ctx context.Context, messageID string) error

	// Reject rejects a message
	Reject(ctx context.Context, messageID string, requeue bool) error
}

// EventPublisher defines the interface for event publishing
type EventPublisher interface {
	// PublishMintEvent publishes a mint event
	PublishMintEvent(ctx context.Context, exchange, routingKey string, event interface{}) error

	// PublishCollectionEvent publishes a collection event
	PublishCollectionEvent(ctx context.Context, exchange, routingKey string, event interface{}) error

	// PublishDomainEvent publishes a domain event
	PublishDomainEvent(ctx context.Context, exchange, routingKey string, event interface{}) error
}
