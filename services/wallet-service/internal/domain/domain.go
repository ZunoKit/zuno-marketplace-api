package domain

import (
	"context"
	"time"
)

type UserID = string
type WalletID = string
type Address = string
type ChainID = string // CAIP-2 format

type WalletLink struct {
	ID         WalletID
	UserID     UserID
	AccountID  string
	Address    Address // lowercase 0x...
	ChainID    ChainID // CAIP-2 format (e.g., "eip155:1")
	IsPrimary  bool
	Type       WalletType // EOA or Contract
	Connector  string     // metamask, walletconnect, etc.
	Label      string     // user-defined label
	VerifiedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type WalletType string

const (
	WalletTypeEOA      WalletType = "eoa"
	WalletTypeContract WalletType = "contract"
)

type WalletLinkedEvent struct {
	UserID         UserID
	WalletID       WalletID
	Address        Address
	ChainID        ChainID
	IsPrimary      bool
	PrimaryChanged bool
	LinkedAt       time.Time
}

type WalletService interface {
	UpsertLink(ctx context.Context, userID UserID, accountID string, address Address, chainID ChainID, isPrimary bool, walletType, connector, label string) (*WalletLink, bool, bool, error)
	GetUserWallets(ctx context.Context, userID UserID) ([]*WalletLink, error)
	GetWalletByAddress(ctx context.Context, address Address) (*WalletLink, error)
	SetPrimaryWallet(ctx context.Context, userID UserID, walletID WalletID) error
	RemoveWallet(ctx context.Context, userID UserID, walletID WalletID) error
}

type WalletRepository interface {
	CreateLink(ctx context.Context, link *WalletLink) error
	GetLink(ctx context.Context, walletID WalletID) (*WalletLink, error)
	GetLinkByUserAndAddress(ctx context.Context, userID UserID, address Address) (*WalletLink, error)
	GetUserWallets(ctx context.Context, userID UserID) ([]*WalletLink, error)
	GetWalletByAddress(ctx context.Context, address Address) (*WalletLink, error)
	UpdateLink(ctx context.Context, link *WalletLink) error
	UpdatePrimaryWallet(ctx context.Context, userID UserID, walletID WalletID) error
	DeleteLink(ctx context.Context, walletID WalletID) error
}

type WalletEventPublisher interface {
	PublishWalletLinked(ctx context.Context, event *WalletLinkedEvent) error
}
