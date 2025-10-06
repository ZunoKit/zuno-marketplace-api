package contracts

import (
	"context"
)

// AMQPMessage represents a message to be published to AMQP
type AMQPMessage struct {
	Exchange   string                 `json:"exchange"`
	RoutingKey string                 `json:"routing_key"`
	Body       []byte                 `json:"body"`
	Headers    map[string]interface{} `json:"headers,omitempty"`
}

// AMQPClient defines the interface for AMQP operations
type AMQPClient interface {
	// Publish publishes a message to the specified exchange
	Publish(ctx context.Context, message AMQPMessage) error

	// Close closes the AMQP connection
	Close() error
}

// Exchange names - configurable constants
const (
	AuthExchange        = "auth.events"
	WalletsExchange     = "wallets.events"
	UsersExchange       = "users.events"
	CollectionsExchange = "collections.events"
	MintsExchange       = "mints.events"
	DLXExchange         = "dlx.events"
)

// Queue names - configurable constants
const (
	// Auth queues
	AuthLoggedInQueue = "subs.auth.logged_in"

	// Wallet queues
	WalletsLinkedQueue = "subs.wallets.linked"

	// Collection queues
	CollectionsCreatedQueue  = "catalog.collections.created"
	CollectionsUpsertedQueue = "subs.collections.upserted"

	// Mint queues
	MintsCreatedQueue  = "catalog.mints.created"
	MintsUpsertedQueue = "subs.mints.upserted"
)

// Routing keys - configurable constants
const (
	// Auth routing keys
	UserLoggedInKey = "user.logged_in"

	// Wallet routing keys
	WalletLinkedKey    = "wallet.linked"
	WalletUnlinkedKey  = "wallet.unlinked"
	ApprovalUpdatedKey = "approval.updated"

	// Collection routing keys
	CollectionCreatedKeyPattern  = "created.eip155.*" // created.eip155.{chainNum}
	CollectionUpsertedKeyPattern = "upserted.*"       // upserted.{chainId}.{contract}

	// Mint routing keys
	MintCreatedKeyPattern  = "minted.eip155.*" // minted.eip155.{chainNum}
	MintUpsertedKeyPattern = "upserted.*"      // upserted.{chainId}.{contract}.{tokenId}
)
