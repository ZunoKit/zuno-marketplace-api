package domain

import (
	"context"
	"math/big"
	"time"
)

type ChainID string

type Address string // EIP-55 normalized

type Standard string // ERC721 | ERC1155 | Custom

type Collection struct {
	ID                string    `db:"id" json:"id"`
	Slug              string    `db:"slug" json:"slug"`
	Name              string    `db:"name" json:"name"`
	Description       string    `db:"description" json:"description"`
	ChainID           string    `db:"chain_id" json:"chain_id"`
	ContractAddress   string    `db:"contract_address" json:"contract_address"`
	Creator           string    `db:"creator" json:"creator"`
	TxHash            string    `db:"tx_hash" json:"tx_hash"`
	Owner             string    `db:"owner" json:"owner"`
	CollectionType    string    `db:"collection_type" json:"collection_type"` // ERC721, ERC1155
	MaxSupply         *big.Int  `db:"max_supply" json:"max_supply"`
	TotalSupply       *big.Int  `db:"total_supply" json:"total_supply"`
	RoyaltyRecipient  string    `db:"royalty_recipient" json:"royalty_recipient"`
	RoyaltyPercentage uint16    `db:"royalty_percentage" json:"royalty_percentage"`
	
	// Minting Configuration Fields from CollectionParams
	MintPrice              *big.Int `db:"mint_price" json:"mint_price"`
	RoyaltyFee             *big.Int `db:"royalty_fee" json:"royalty_fee"`
	MintLimitPerWallet     *big.Int `db:"mint_limit_per_wallet" json:"mint_limit_per_wallet"`
	MintStartTime          *big.Int `db:"mint_start_time" json:"mint_start_time"`
	AllowlistMintPrice     *big.Int `db:"allowlist_mint_price" json:"allowlist_mint_price"`
	PublicMintPrice        *big.Int `db:"public_mint_price" json:"public_mint_price"`
	AllowlistStageDuration *big.Int `db:"allowlist_stage_duration" json:"allowlist_stage_duration"`
	TokenURI               string   `db:"token_uri" json:"token_uri"`
	
	IsVerified        bool      `db:"is_verified" json:"is_verified"`
	IsExplicit        bool      `db:"is_explicit" json:"is_explicit"`
	IsFeatured        bool      `db:"is_featured" json:"is_featured"`
	ImageURL          string    `db:"image_url" json:"image_url"`
	BannerURL         string    `db:"banner_url" json:"banner_url"`
	ExternalURL       string    `db:"external_url" json:"external_url"`
	DiscordURL        string    `db:"discord_url" json:"discord_url"`
	TwitterURL        string    `db:"twitter_url" json:"twitter_url"`
	InstagramURL      string    `db:"instagram_url" json:"instagram_url"`
	TelegramURL       string    `db:"telegram_url" json:"telegram_url"`
	FloorPrice        *big.Int  `db:"floor_price" json:"floor_price"`
	VolumeTraded      *big.Int  `db:"volume_traded" json:"volume_traded"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time `db:"updated_at" json:"updated_at"`
}

// ProcessedEvent tracks which events have been processed to ensure idempotency
type ProcessedEvent struct {
	EventID     string    `db:"event_id" json:"event_id"`
	EventType   string    `db:"event_type" json:"event_type"`
	ChainID     string    `db:"chain_id" json:"chain_id"`
	TxHash      string    `db:"tx_hash" json:"tx_hash"`
	ProcessedAt time.Time `db:"processed_at" json:"processed_at"`
}

// CollectionEvent represents an incoming collection event from the indexer
type CollectionEvent struct {
	Schema    string                 `json:"schema"`
	Version   string                 `json:"version"`
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	ChainID   string                 `json:"chain_id"`
	TxHash    string                 `json:"tx_hash"`
	Contract  string                 `json:"contract"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// DomainEvent represents a domain event published by the catalog service
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

// CollectionEventHandler handles collection events
type CollectionEventHandler func(ctx context.Context, event *CollectionEvent) error

type CatalogService interface {
	HandleCollectionCreated(ctx context.Context, evt *CollectionEvent) error
}

type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}

type Tx interface {
	CollectionsRepo() CollectionsRepository
	ProcessedRepo() ProcessedEventsRepository
}

type CollectionsRepository interface {
	Upsert(ctx context.Context, c Collection) (created bool, err error)

	GetByPK(ctx context.Context, chainID ChainID, contract Address) (Collection, error)
}

type ProcessedEventsRepository interface {
	MarkProcessed(ctx context.Context, eventID string) (bool, error)
}

type MessagePublisher interface {
	PublishCollectionUpserted(ctx context.Context, collection *Collection) error
	PublishDomainEvent(ctx context.Context, event *DomainEvent) error
}
