package domain

import (
	"context"
	"time"
)

type UserID = string
type SessionID = string
type Address = string
type ChainID = string // CAIP-2

type AuthUserLoggedInEvent struct {
	UserID     UserID
	AccountID  string
	Address    Address
	ChainID    ChainID
	SessionID  SessionID
	LoggedInAt time.Time
}

type Nonce struct {
	Value     string
	AccountID string
	ChainID   ChainID
	Domain    string
	Used      bool
	ExpiresAt time.Time
	CreatedAt time.Time
}

type Session struct {
	ID          SessionID
	UserID      UserID
	RefreshHash string // HASH(refreshToken)
	ExpiresAt   time.Time
	CreatedAt   time.Time
	RevokedAt   *time.Time
	DeviceID    *string
	IP          *string
	UA          *string
	LastUsedAt  *time.Time
	// Optional JSON context for collection preparation, stored as JSON string
	CollectionIntentContext *string
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	UserID       UserID
	Address      Address
	ChainID      ChainID
}

type AuthService interface {
	GetNonce(ctx context.Context, accountID, chainID, domain string) (string, error)
	VerifySiwe(ctx context.Context, accountID, message, signature string) (*AuthResult, error)
	Refresh(ctx context.Context, refreshToken string) (*AuthResult, error)
	Logout(ctx context.Context, sessionID string) error
	LogoutByRefreshToken(ctx context.Context, refreshToken string) error
}

type AuthEventPublisher interface {
	PublishUserLoggedIn(ctx context.Context, event *AuthUserLoggedInEvent) error
}

type AuthRepository interface {

	// Nonce
	CreateNonce(ctx context.Context, nonce *Nonce) error
	GetNonce(ctx context.Context, value string) (*Nonce, error)
	TryUseNonce(ctx context.Context, value, accountID, chainID, domain string, usedAt time.Time) (bool, error)

	// Session
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID SessionID) (*Session, error)
	GetSessionByRefreshHash(ctx context.Context, refreshHash string) (*Session, error)
	UpdateSessionLastUsed(ctx context.Context, sessionID SessionID) error
	RevokeSession(ctx context.Context, sessionID SessionID) error
}
