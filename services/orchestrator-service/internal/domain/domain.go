package domain

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Address = string
type ChainID = string
type Sha256 = string

type IntentKind string

const (
	IntentKindCollection IntentKind = "collection"
	IntentKindMint       IntentKind = "mint"
)

type IntentStatus string

const (
	IntentPending IntentStatus = "pending"
	IntentReady   IntentStatus = "ready"
	IntentFailed  IntentStatus = "failed"
	IntentExpired IntentStatus = "expired"
)

type Standard string

const (
	StdCustom  Standard = "CUSTOM"
	StdERC721  Standard = "ERC721"
	StdERC1155 Standard = "ERC1155"
	StdProxy   Standard = "PROXY"
	StdDiamond Standard = "DIAMOND"
)

type Intent struct {
	ID              string       `json:"id"`
	Kind            IntentKind   `json:"kind"`
	ChainID         ChainID      `json:"chainId"`
	PreviewAddress  *Address     `json:"previewAddress,omitempty"`
	ContractAddress *Address     `json:"contractAddress,omitempty"` // helpful for mint/intents
	TxHash          *string      `json:"txHash,omitempty"`
	Status          IntentStatus `json:"status"`
	CreatedBy       *string      `json:"createdBy,omitempty"`      // user id (uuid)
	ReqPayloadJSON  any          `json:"reqPayloadJson,omitempty"` // decoded view; persist as JSONB
	Error           *string      `json:"error,omitempty"`
	AuthSessionID   *string      `json:"authSessionId,omitempty"`
	DeadlineAt      *time.Time   `json:"deadlineAt,omitempty"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}

type TxRequest struct {
	To             Address  `json:"to"`
	Data           []byte   `json:"data"`           // raw calldata; transport may hex-encode
	Value          string   `json:"value"`          // wei as decimal string (or "0")
	PreviewAddress *Address `json:"previewAddress"` // optional address FE can show as preview
}

// WS/Subscription payload
type IntentStatusPayload struct {
	IntentID        string       `json:"intentId"`
	Kind            IntentKind   `json:"kind"`
	Status          IntentStatus `json:"status"`
	ChainID         *ChainID     `json:"chainId,omitempty"`
	TxHash          *string      `json:"txHash,omitempty"`
	ContractAddress *Address     `json:"contractAddress,omitempty"`
}

type PrepareCreateCollectionInput struct {
	ChainID                ChainID  `json:"chainId"`
	Name                   string   `json:"name"`
	Symbol                 string   `json:"symbol"`
	Creator                Address  `json:"creator"`
	TokenURI               string   `json:"tokenURI"`
	Type                   Standard `json:"type"` // ERC721 or ERC1155 - specifies the collection type
	Description            *string  `json:"description,omitempty"`
	MintPrice              *uint64  `json:"mintPrice,omitempty"`
	RoyaltyFee             *uint64  `json:"royaltyFee,omitempty"`
	MaxSupply              *uint64  `json:"maxSupply,omitempty"`
	MintLimitPerWallet     *uint64  `json:"mintLimitPerWallet,omitempty"`
	MintStartTime          *uint64  `json:"mintStartTime,omitempty"`
	AllowlistMintPrice     *uint64  `json:"allowlistMintPrice,omitempty"`
	PublicMintPrice        *uint64  `json:"publicMintPrice,omitempty"`
	AllowlistStageDuration *uint64  `json:"allowlistStageDuration,omitempty"`

	CreatedBy  *string    `json:"createdBy,omitempty"`
	DeadlineAt *time.Time `json:"deadlineAt,omitempty"`
	ReqMeta    any        `json:"reqMeta,omitempty"` // kept in req_payload_json
}

type PrepareCreateCollectionResult struct {
	IntentID string    `json:"intentId"`
	Tx       TxRequest `json:"txRequest"`
}

// Mint

type PrepareMintInput struct {
	ChainID   ChainID  `json:"chainId"`
	Contract  Address  `json:"contract"`
	Standard  Standard `json:"standard"` // ERC721 | ERC1155
	Minter    Address  `json:"minter"`
	Quantity  uint64   `json:"quantity"` // ERC721: 1
	TokenID   *uint64  `json:"tokenId,omitempty"`
	CreatedBy *string  `json:"createdBy,omitempty"`
	ReqMeta   any      `json:"reqMeta,omitempty"`
}

type PrepareMintResult struct {
	IntentID string    `json:"intentId"`
	Tx       TxRequest `json:"txRequest"`
}

type TrackTxInput struct {
	IntentID       string   `json:"intentId"`
	ChainID        ChainID  `json:"chainId"`
	TxHash         string   `json:"txHash"`
	Contract       *Address `json:"contract,omitempty"`       // useful for mint
	PreviewAddress *Address `json:"previewAddress,omitempty"` // optional echo
}

type OrchestratorRepo interface {
	Create(ctx context.Context, it *Intent) error
	UpdateTxHash(ctx context.Context, intentID string, txHash string, contractAddr *Address) error
	UpdateStatus(ctx context.Context, intentID string, status IntentStatus, errMsg *string) error
	GetByID(ctx context.Context, intentID string) (*Intent, error)

	FindByChainTx(ctx context.Context, chainID ChainID, txHash string) (*Intent, error)
	InsertSessionIntentAudit(ctx context.Context, sessionID string, intentID string, userID *string, auditData any) error
}

type StatusCache interface {
	SetIntentStatus(ctx context.Context, payload IntentStatusPayload, ttl time.Duration) error
}

type Encoder interface {
	EncodeCreateCollection(ctx context.Context, chainID ChainID, factory Address, p PrepareCreateCollectionInput) (to Address, data []byte, value string, preview *Address, err error)

	EncodeMint(ctx context.Context, chainID ChainID, contract Address, standard Standard, p PrepareMintInput) (to Address, data []byte, value string, err error)
}

type OrchestratorService interface {
	PrepareCreateCollection(ctx context.Context, in PrepareCreateCollectionInput) (*PrepareCreateCollectionResult, error)
	PrepareMint(ctx context.Context, in PrepareMintInput) (*PrepareMintResult, error)

	TrackTx(ctx context.Context, in TrackTxInput) (ok bool, err error)

	GetIntentStatus(ctx context.Context, intentID string) (*IntentStatusPayload, error)
}

const DefaultIntentTTL = 6 * time.Hour

type CollectionParams struct {
	Name                   string
	Symbol                 string
	Owner                  common.Address
	Description            string
	MintPrice              *big.Int
	RoyaltyFee             *big.Int
	MaxSupply              *big.Int
	MintLimitPerWallet     *big.Int
	MintStartTime          *big.Int
	AllowlistMintPrice     *big.Int
	PublicMintPrice        *big.Int
	AllowlistStageDuration *big.Int
	TokenURI               string
}
