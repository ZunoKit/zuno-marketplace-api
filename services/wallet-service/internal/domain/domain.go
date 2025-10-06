package domain

import (
	"context"
	"errors"
	"time"
)

// Type aliases for better readability
type WalletID = string
type AccountID = string
type UserID = string
type Address = string
type ChainID = string // CAIP-2 format

type WalletLink struct {
	ID         WalletID
	UserID     UserID
	AccountID  AccountID
	Address    Address
	ChainID    ChainID
	IsPrimary  bool
	VerifiedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type WalletUpsertResult struct {
	Link           *WalletLink
	Created        bool
	PrimaryChanged bool
}

type WalletService interface {
	UpsertLink(ctx context.Context, link WalletLink) (*WalletUpsertResult, error)
}

// WalletRepository defines the data persistence interface
type WalletRepository interface {
	WithTx(ctx context.Context, fn func(TxWalletRepository) error) error
}

type TxWalletRepository interface {
	// Khóa để tránh race song song
	AcquireAccountLock(ctx context.Context, accountID string) error
	AcquireAddressLock(ctx context.Context, chainID, address string) error

	// Truy vấn tồn tại
	GetByAccountIDTx(ctx context.Context, accountID string) (*WalletLink, error)      // ErrWalletNotFound nếu không có
	GetByAddressTx(ctx context.Context, chainID, address string) (*WalletLink, error) // ErrWalletNotFound nếu không có

	// Ghi/Update
	InsertWalletTx(ctx context.Context, link WalletLink) (*WalletLink, error)
	UpdateWalletMetaTx(ctx context.Context, id WalletID, isPrimary *bool, verifiedAt *time.Time, lastSeen *time.Time, label *string) (*WalletLink, error)

	// Primary logic
	GetPrimaryByUserTx(ctx context.Context, userID UserID) (*WalletLink, error) // ErrWalletNotFound nếu chưa có

	GetPrimaryByUserChainTx(ctx context.Context, userID UserID, chainID ChainID) (*WalletLink, error)
	DemoteOtherPrimariesTx(ctx context.Context, userID UserID, chainID ChainID, keepID WalletID) error
	UpdateWalletAddressTx(ctx context.Context, id WalletID, chainID ChainID, address Address) (*WalletLink, error)
}

type EventPublisher interface {
	PublishWalletLinked(ctx context.Context, event *WalletLinkedEvent) error
}

type WalletLinkedEvent struct {
	UserID    UserID    `json:"user_id"`
	AccountID AccountID `json:"account_id"`
	WalletID  WalletID  `json:"wallet_id"`
	Address   Address   `json:"address"`
	ChainID   ChainID   `json:"chain_id"`
	IsPrimary bool      `json:"is_primary"`
	LinkedAt  time.Time `json:"linked_at"`
}

// Error definitions
var (
	ErrWalletNotFound      = errors.New("wallet_not_found")
	ErrWalletAlreadyExists = errors.New("wallet_already_exists")
	ErrUnauthorizedAccess  = errors.New("unauthorized_access")
	ErrInvalidChainID      = errors.New("invalid_chain_id")
	ErrInvalidAddress      = errors.New("invalid_address")
	ErrCannotRemovePrimary = errors.New("cannot_remove_primary_wallet")
	ErrApprovalNotFound    = errors.New("approval_not_found")
	ErrInvalidStandard     = errors.New("invalid_token_standard")
)
