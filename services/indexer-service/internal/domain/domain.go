package domain

import (
	"context"
	"math/big"
	"time"
)

// RawEvent represents a raw blockchain event stored in MongoDB
type RawEvent struct {
	ID              string                 `bson:"_id" json:"id"`
	ChainID         string                 `bson:"chain_id" json:"chain_id"`
	TxHash          string                 `bson:"tx_hash" json:"tx_hash"`
	LogIndex        int                    `bson:"log_index" json:"log_index"`
	BlockNumber     *big.Int               `bson:"block_number" json:"block_number"`
	BlockHash       string                 `bson:"block_hash" json:"block_hash"`
	ContractAddress string                 `bson:"contract_address" json:"contract_address"`
	EventName       string                 `bson:"event_name" json:"event_name"`
	EventSignature  string                 `bson:"event_signature" json:"event_signature"`
	RawData         map[string]interface{} `bson:"raw_data" json:"raw_data"`
	ParsedJSON      string                 `bson:"parsed_json" json:"parsed_json"`
	Confirmations   int                    `bson:"confirmations" json:"confirmations"`
	ObservedAt      time.Time              `bson:"observed_at" json:"observed_at"`
	CreatedAt       time.Time              `bson:"created_at" json:"created_at"`
}

// Checkpoint represents the indexing progress for a specific chain
type Checkpoint struct {
	ChainID       string    `db:"chain_id" json:"chain_id"`
	LastBlock     *big.Int  `db:"last_block" json:"last_block"`
	LastBlockHash string    `db:"last_block_hash" json:"last_block_hash"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
}

// CollectionCreatedEvent represents the parsed CollectionCreated event
type CollectionCreatedEvent struct {
	CollectionAddress string   `json:"collection_address"`
	Creator           string   `json:"creator"`
	Name              string   `json:"name"`
	Symbol            string   `json:"symbol"`
	CollectionType    string   `json:"collection_type"` // "ERC721" or "ERC1155"
	MaxSupply         *big.Int `json:"max_supply"`
	RoyaltyRecipient  string   `json:"royalty_recipient"`
	RoyaltyPercentage uint16   `json:"royalty_percentage"`
}

// PublishableEvent represents an event ready to be published to RabbitMQ
type PublishableEvent struct {
	Schema    string                 `json:"schema"`
	Version   string                 `json:"version"`
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"` // "collection_created"
	ChainID   string                 `json:"chain_id"`
	TxHash    string                 `json:"tx_hash"`
	Contract  string                 `json:"contract"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// Repository interfaces

type EventRepository interface {
	// StoreRawEvent stores a raw blockchain event with deduplication
	StoreRawEvent(ctx context.Context, event *RawEvent) error

	// GetRawEvent retrieves a raw event by unique key
	GetRawEvent(ctx context.Context, chainID, txHash string, logIndex int) (*RawEvent, error)

	// GetEventsByBlock retrieves all events for a specific block
	GetEventsByBlock(ctx context.Context, chainID string, blockNumber *big.Int) ([]*RawEvent, error)
}

type CheckpointRepository interface {
	// GetCheckpoint retrieves the latest checkpoint for a chain
	GetCheckpoint(ctx context.Context, chainID string) (*Checkpoint, error)

	// UpdateCheckpoint updates the checkpoint for a chain
	UpdateCheckpoint(ctx context.Context, checkpoint *Checkpoint) error

	// CreateCheckpoint creates a new checkpoint for a chain
	CreateCheckpoint(ctx context.Context, checkpoint *Checkpoint) error

	// HealthCheck performs a health check on the repository
	HealthCheck(ctx context.Context) error
}

// Service interfaces

type EventPublisher interface {
	// PublishCollectionEvent publishes a collection-related event
	PublishCollectionEvent(ctx context.Context, chainID string, event *PublishableEvent) error

	// PublishCollectionCreatedEvent publishes a CollectionCreated event
	PublishCollectionCreatedEvent(ctx context.Context, chainID string, rawEvent *RawEvent, collectionEvent *CollectionCreatedEvent) error
}

type BlockchainClient interface {
	// GetLatestBlock returns the latest block number
	GetLatestBlock(ctx context.Context) (*big.Int, error)

	// GetBlockByNumber returns block information
	GetBlockByNumber(ctx context.Context, blockNumber *big.Int) (*BlockInfo, error)

	// GetLogs returns logs for the specified filter
	GetLogs(ctx context.Context, filter *LogFilter) ([]*Log, error)

	// GetConfirmations returns the number of confirmations for a block
	GetConfirmations(ctx context.Context, blockNumber *big.Int) (int, error)

	// ParseCollectionCreatedLog parses a CollectionCreated log
	ParseCollectionCreatedLog(log *Log) (*CollectionCreatedEvent, error)

	// IsHealthy checks if the blockchain client is healthy
	IsHealthy(ctx context.Context) error
}

// Blockchain types

type BlockInfo struct {
	Number    *big.Int  `json:"number"`
	Hash      string    `json:"hash"`
	Timestamp time.Time `json:"timestamp"`
}

type LogFilter struct {
	FromBlock *big.Int `json:"from_block"`
	ToBlock   *big.Int `json:"to_block"`
	Addresses []string `json:"addresses"`
	Topics    []string `json:"topics"`
}

type Log struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber *big.Int `json:"block_number"`
	TxHash      string   `json:"tx_hash"`
	LogIndex    int      `json:"log_index"`
	BlockHash   string   `json:"block_hash"`
	Removed     bool     `json:"removed"`
}

// IndexerService interface
type IndexerService interface {
	// Start begins the indexing process for all configured chains
	Start(ctx context.Context) error

	// Stop gracefully shuts down the indexing process
	Stop(ctx context.Context) error

	// IndexChain indexes events for a specific chain
	IndexChain(ctx context.Context, chainID string) error
}
