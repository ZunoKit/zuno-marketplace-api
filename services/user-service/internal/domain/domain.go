package domain

import (
	"context"
	"time"
)

type UserID = string
type AccountID = string // wallet account ID
type Address = string   // ethereum address
type ChainID = string   // CAIP-2

// User represents the core user entity
type User struct {
	ID        UserID
	Status    string // active, suspended, etc.
	CreatedAt time.Time
}

type Profile struct {
	UserID      UserID
	Username    string
	DisplayName string
	AvatarURL   string
	BannerURL   string
	Bio         string
	Locale      string
	Timezone    string
	SocialsJSON string // JSON string containing social media links
	UpdatedAt   time.Time
}

type EnsureUserResult struct {
	UserID  UserID
	Created bool // true if new user was created
}

type UserService interface {
	EnsureUser(ctx context.Context, accountID AccountID, address Address, chainID ChainID) (*EnsureUserResult, error)
}

type UserRepository interface {
	GetUserIDByAccount(ctx context.Context, accountID string) (string, error)

	WithTx(ctx context.Context, fn func(TxUserRepository) error) error
}

type TxUserRepository interface {
	AcquireAccountLock(ctx context.Context, accountID string) error
	GetUserIDByAccountTx(ctx context.Context, accountID string) (string, error)

	CreateUserTx(ctx context.Context) (string, error)
	CreateProfileTx(ctx context.Context, userID string) error

	UpsertUserAccountTx(ctx context.Context, userID, accountID, address, chainID string) error
	TouchUserAccountTx(ctx context.Context, accountID, address string) error
}

// Validation helpers
func ValidateAccountID(accountID AccountID) error {
	if accountID == "" {
		return NewInvalidInputError("account_id", "cannot be empty")
	}
	if len(accountID) > 255 {
		return NewInvalidInputError("account_id", "too long")
	}
	return nil
}

func ValidateAddress(address Address) error {
	if address == "" {
		return NewInvalidInputError("address", "cannot be empty")
	}
	if len(address) != 42 {
		return NewInvalidInputError("address", "must be 42 characters long")
	}
	if address[:2] != "0x" {
		return NewInvalidInputError("address", "must start with 0x")
	}
	// Check if the rest are hex characters
	for i := 2; i < len(address); i++ {
		c := address[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return NewInvalidInputError("address", "contains invalid hex characters")
		}
	}
	return nil
}

func ValidateChainID(chainID ChainID) error {
	if chainID == "" {
		return NewInvalidInputError("chain_id", "cannot be empty")
	}
	return nil
}
