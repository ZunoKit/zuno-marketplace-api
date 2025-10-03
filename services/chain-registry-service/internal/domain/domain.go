package domain

import (
	"context"
	"time"
)

// ---------- Strong types ----------
type Address = string // lowercase 0x...; normalize ở layer repo
type ChainID = string // CAIP-2: e.g. "eip155:8453"
type Sha256 = string

// ---------- Enums ----------
type ContractStandard string

const (
	StdCustom  ContractStandard = "CUSTOM"
	StdERC721  ContractStandard = "ERC721"
	StdERC1155 ContractStandard = "ERC1155"
	StdProxy   ContractStandard = "PROXY"
	StdDiamond ContractStandard = "DIAMOND"
)

type RpcAuthType string

const (
	RpcAuthNone   RpcAuthType = "NONE"
	RpcAuthKey    RpcAuthType = "KEY"
	RpcAuthBasic  RpcAuthType = "BASIC"
	RpcAuthBearer RpcAuthType = "BEARER"
)

// ---------- Models ----------
type Contract struct {
	Name        string           `json:"name"`
	Address     Address          `json:"address"`
	StartBlock  uint64           `json:"startBlock,omitempty"`
	VerifiedAt  *time.Time       `json:"verifiedAt,omitempty"`
	Standard    ContractStandard `json:"standard,omitempty"`
	ImplAddress *Address         `json:"implAddress,omitempty"` // nếu là proxy
	AbiSHA256   *Sha256          `json:"abiSha256,omitempty"`   // content-addressed
}

type GasPolicy struct {
	// Ghi chú: float64 OK cho Gwei (không phải tiền on-chain), nếu cần tuyệt đối: chuyển sang decimal lib.
	MaxFeeGwei              float64   `json:"maxFeeGwei"`
	PriorityFeeGwei         float64   `json:"priorityFeeGwei"`
	Multiplier              float64   `json:"multiplier"`
	LastObservedBaseFeeGwei float64   `json:"lastObservedBaseFeeGwei,omitempty"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

type RpcEndpoint struct {
	URL       string      `json:"url"`
	Priority  int32       `json:"priority"` // lower => higher priority
	Weight    int32       `json:"weight"`
	AuthType  RpcAuthType `json:"authType"`
	RateLimit *int32      `json:"rateLimit,omitempty"`
	Active    bool        `json:"active"`
}

type ChainParams struct {
	RequiredConfirmations uint32 `json:"requiredConfirmations"`
	ReorgDepth            uint32 `json:"reorgDepth"` // ví dụ 12
	BlockTimeMs           uint32 `json:"blockTimeMs,omitempty"`
}

type ChainContracts struct {
	ChainID         ChainID     `json:"chainId"`
	ChainNumeric    uint64      `json:"chainNumeric"`
	NativeSymbol    string      `json:"nativeSymbol,omitempty"`
	Contracts       []Contract  `json:"contracts"`
	Params          ChainParams `json:"params"`
	RegistryVersion string      `json:"registryVersion"`
}

type ChainGasPolicy struct {
	ChainID         ChainID   `json:"chainId"`
	Policy          GasPolicy `json:"policy"`
	RegistryVersion string    `json:"registryVersion"`
}

type ChainRpcEndpoints struct {
	ChainID         ChainID       `json:"chainId"`
	Endpoints       []RpcEndpoint `json:"endpoints"`
	RegistryVersion string        `json:"registryVersion"`
}

type ContractMeta struct {
	ChainID         ChainID  `json:"chainId"`
	Contract        Contract `json:"contract"`
	RegistryVersion string   `json:"registryVersion"`
}

// ---------- Ports ----------
type ChainRegistryRepository interface {
	GetContracts(ctx context.Context, chainID ChainID) (*ChainContracts, error)
	GetGasPolicy(ctx context.Context, chainID ChainID) (*ChainGasPolicy, error)
	GetRpcEndpoints(ctx context.Context, chainID ChainID) (*ChainRpcEndpoints, error)
	BumpVersion(ctx context.Context, chainID ChainID, reason string) (newVersion string, err error)

	GetContractMeta(ctx context.Context, chainID ChainID, address Address) (*ContractMeta, error)
	GetAbiBlob(ctx context.Context, sha Sha256) (abiJSON []byte, etag string, err error)
	ResolveProxy(ctx context.Context, chainID ChainID, address Address) (implAddress Address, abiSha256 Sha256, err error)
}

type ChainRegistryService interface {
	GetContracts(ctx context.Context, chainID ChainID) (*ChainContracts, error)
	GetGasPolicy(ctx context.Context, chainID ChainID) (*ChainGasPolicy, error)
	GetRpcEndpoints(ctx context.Context, chainID ChainID) (*ChainRpcEndpoints, error)
	BumpVersion(ctx context.Context, chainID ChainID, reason string) (ok bool, newVersion string, err error)

	GetContractMeta(ctx context.Context, chainID ChainID, address Address) (*ContractMeta, error)
	GetAbiBlob(ctx context.Context, sha Sha256) (abiJSON []byte, etag string, err error)
	ResolveProxy(ctx context.Context, chainID ChainID, address Address) (implAddress Address, abiSha256 Sha256, err error)

	// Friendly API: fetch ABI directly by chain + address
	GetAbiByAddress(ctx context.Context, chainID ChainID, address Address) (abiJSON []byte, etag string, err error)
}
