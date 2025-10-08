package domain

import (
	"context"
	"fmt"
	"time"
)

// NFT represents a non-fungible token in the catalog
type NFT struct {
	ID         string       `json:"id" bson:"_id"`
	ChainID    string       `json:"chain_id" bson:"chain_id"`
	Contract   string       `json:"contract" bson:"contract"`
	TokenID    string       `json:"token_id" bson:"token_id"`
	Owner      string       `json:"owner" bson:"owner"`
	TokenURI   string       `json:"token_uri" bson:"token_uri"`
	Standard   string       `json:"standard" bson:"standard"` // ERC721 or ERC1155
	Supply     string       `json:"supply" bson:"supply"`     // For ERC1155
	MetadataID *string      `json:"metadata_id" bson:"metadata_id"`
	Metadata   *NFTMetadata `json:"metadata,omitempty" bson:"-"` // Loaded from metadata repo
	CreatedAt  time.Time    `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" bson:"updated_at"`
}

// NFTMetadata represents normalized NFT metadata
type NFTMetadata struct {
	Name            string                 `json:"name" bson:"name"`
	Description     string                 `json:"description" bson:"description"`
	Image           string                 `json:"image" bson:"image"`
	AnimationURL    string                 `json:"animation_url,omitempty" bson:"animation_url"`
	ExternalURL     string                 `json:"external_url,omitempty" bson:"external_url"`
	BackgroundColor string                 `json:"background_color,omitempty" bson:"background_color"`
	Attributes      []Attribute            `json:"attributes" bson:"attributes"`
	Properties      map[string]interface{} `json:"properties,omitempty" bson:"properties"`
}

// Attribute represents an NFT attribute/trait
type Attribute struct {
	TraitType   string  `json:"trait_type" bson:"trait_type"`
	Value       string  `json:"value" bson:"value"`
	DisplayType *string `json:"display_type,omitempty" bson:"display_type"`
	MaxValue    *string `json:"max_value,omitempty" bson:"max_value"`
}

// MetadataDocument represents a metadata document in MongoDB
type MetadataDocument struct {
	ID         string       `json:"id" bson:"_id"`
	ChainID    string       `json:"chain_id" bson:"chain_id"`
	Contract   string       `json:"contract" bson:"contract"`
	TokenID    string       `json:"token_id" bson:"token_id"`
	Normalized *NFTMetadata `json:"normalized" bson:"normalized"`
	RawJSON    string       `json:"raw_json,omitempty" bson:"raw_json"`
	Media      *MediaInfo   `json:"media,omitempty" bson:"media"`
	CreatedAt  time.Time    `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" bson:"updated_at"`
}

// MediaInfo represents processed media information
type MediaInfo struct {
	Image        string `json:"image" bson:"image"`
	ImageCDN     string `json:"image_cdn,omitempty" bson:"image_cdn"`
	AnimationURL string `json:"animation_url,omitempty" bson:"animation_url"`
	AnimationCDN string `json:"animation_cdn,omitempty" bson:"animation_cdn"`
	Processed    bool   `json:"processed" bson:"processed"`
}

// ProcessedEvent represents a processed event for idempotency
type ProcessedEvent struct {
	ID          string    `json:"id" bson:"_id"`
	EventID     string    `json:"event_id" bson:"event_id"`
	TxHash      string    `json:"tx_hash" bson:"tx_hash"`
	ProcessedAt time.Time `json:"processed_at" bson:"processed_at"`
}

// NFTFilter represents filter criteria for listing NFTs
type NFTFilter struct {
	ChainID  *string
	Contract *string
	Owner    *string
	Standard *string
	Limit    int
	Offset   int
	SortBy   string // created_at, updated_at, token_id
	SortDir  string // asc, desc
}

// Repository interfaces

// NFTRepository defines the interface for NFT storage
type NFTRepository interface {
	// UpsertNFT creates or updates an NFT
	UpsertNFT(ctx context.Context, nft *NFT) error

	// GetNFT retrieves a specific NFT
	GetNFT(ctx context.Context, chainID, contract, tokenID string) (*NFT, error)

	// ListNFTs lists NFTs with filtering and pagination
	ListNFTs(ctx context.Context, filter *NFTFilter) ([]*NFT, error)

	// CountNFTs counts NFTs matching the filter
	CountNFTs(ctx context.Context, filter *NFTFilter) (int64, error)

	// UpdateOwner updates the owner of an NFT
	UpdateOwner(ctx context.Context, chainID, contract, tokenID, newOwner string) error

	// DeleteNFT removes an NFT from the catalog
	DeleteNFT(ctx context.Context, chainID, contract, tokenID string) error
}

// MetadataRepository defines the interface for metadata storage
type MetadataRepository interface {
	// UpsertMetadata creates or updates metadata
	UpsertMetadata(ctx context.Context, metadata *MetadataDocument) error

	// GetMetadata retrieves metadata by ID
	GetMetadata(ctx context.Context, id string) (*MetadataDocument, error)

	// GetMetadataByNFT retrieves metadata for a specific NFT
	GetMetadataByNFT(ctx context.Context, chainID, contract, tokenID string) (*MetadataDocument, error)

	// DeleteMetadata removes metadata
	DeleteMetadata(ctx context.Context, id string) error
}

// ProcessedEventRepository defines the interface for tracking processed events
type ProcessedEventRepository interface {
	// IsProcessed checks if an event has been processed
	IsProcessed(ctx context.Context, eventID string) (bool, error)

	// MarkProcessed marks an event as processed
	MarkProcessed(ctx context.Context, eventID, txHash string) error

	// GetProcessedEvent retrieves a processed event
	GetProcessedEvent(ctx context.Context, eventID string) (*ProcessedEvent, error)

	// CleanupOldEvents removes old processed events
	CleanupOldEvents(ctx context.Context, olderThan time.Time) error
}

// External service interfaces

// MetadataFetcher defines the interface for fetching metadata
type MetadataFetcher interface {
	// GetTokenURI retrieves the token URI from the blockchain
	GetTokenURI(ctx context.Context, chainID, contract, tokenID, standard string) (string, error)

	// FetchFromURI fetches metadata from a URI (HTTP/IPFS)
	FetchFromURI(ctx context.Context, uri string) (map[string]interface{}, error)
}

// EventPublisher defines the interface for publishing events
type EventPublisher interface {
	// Publish publishes an event to a topic
	Publish(ctx context.Context, topic, routingKey string, message interface{}) error
}

// Service errors
var (
	ErrNFTNotFound      = fmt.Errorf("NFT not found")
	ErrMetadataNotFound = fmt.Errorf("metadata not found")
	ErrEventProcessed   = fmt.Errorf("event already processed")
	ErrInvalidFilter    = fmt.Errorf("invalid filter")
)
