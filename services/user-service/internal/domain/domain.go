package domain

import (
	"context"
	"time"
)

type UserID = string

type User struct {
	ID        UserID
	Status    UserStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UserStatus string

const (
	UserStatusActive  UserStatus = "active"
	UserStatusBanned  UserStatus = "banned"
	UserStatusDeleted UserStatus = "deleted"
)

type Profile struct {
	UserID      UserID
	Username    string
	DisplayName string
	AvatarURL   string
	BannerURL   string
	Bio         string
	Locale      string
	Timezone    string
	SocialsJSON string
	UpdatedAt   time.Time
}

type UserCreatedEvent struct {
	UserID    UserID
	AccountID string
	Address   string
	ChainID   string
	CreatedAt time.Time
}

type UserService interface {
	EnsureUser(ctx context.Context, accountID, address, chainID string) (UserID, bool, error)
	GetUser(ctx context.Context, userID UserID) (*User, *Profile, error)
	UpdateProfile(ctx context.Context, profile *Profile) (*Profile, error)
}

type UserRepository interface {
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, userID UserID) (*User, error)
	GetUserByAddress(ctx context.Context, address string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error

	CreateProfile(ctx context.Context, profile *Profile) error
	GetProfile(ctx context.Context, userID UserID) (*Profile, error)
	UpdateProfile(ctx context.Context, profile *Profile) error
}

type UserEventPublisher interface {
	PublishUserCreated(ctx context.Context, event *UserCreatedEvent) error
}
