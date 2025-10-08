package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
)

type Repository struct {
	postgres *postgres.Postgres
}

func NewUserRepository(postgres *postgres.Postgres) domain.UserRepository {
	return &Repository{postgres: postgres}
}

// User operations

func (r *Repository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (user_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err := r.postgres.GetClient().ExecContext(ctx, query,
		user.ID,
		user.Status,
		user.CreatedAt,
		user.UpdatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("user already exists: %w", err)
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *Repository) GetUser(ctx context.Context, userID domain.UserID) (*domain.User, error) {
	query := `
		SELECT user_id, status, created_at, updated_at
		FROM users
		WHERE user_id = $1
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, userID)

	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (r *Repository) GetUserByAddress(ctx context.Context, address string) (*domain.User, error) {
	// Join with wallet_links table to find user by address
	query := `
		SELECT u.user_id, u.status, u.created_at, u.updated_at
		FROM users u
		INNER JOIN wallet_links w ON u.user_id = w.user_id
		WHERE LOWER(w.address) = LOWER($1)
		LIMIT 1
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, address)

	var user domain.User
	err := row.Scan(
		&user.ID,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by address: %w", err)
	}

	return &user, nil
}

func (r *Repository) UpdateUser(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET status = $2, updated_at = $3
		WHERE user_id = $1
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query,
		user.ID,
		user.Status,
		user.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}

// Profile operations

func (r *Repository) CreateProfile(ctx context.Context, profile *domain.Profile) error {
	query := `
		INSERT INTO profiles (user_id, username, display_name, avatar_url, banner_url, bio, locale, timezone, socials_json, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.postgres.GetClient().ExecContext(ctx, query,
		profile.UserID,
		profile.Username,
		profile.DisplayName,
		profile.AvatarURL,
		profile.BannerURL,
		profile.Bio,
		profile.Locale,
		profile.Timezone,
		profile.SocialsJSON,
		profile.UpdatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("profile already exists: %w", err)
		}
		return fmt.Errorf("failed to create profile: %w", err)
	}

	return nil
}

func (r *Repository) GetProfile(ctx context.Context, userID domain.UserID) (*domain.Profile, error) {
	query := `
		SELECT user_id, username, display_name, avatar_url, banner_url, bio, locale, timezone, socials_json, updated_at
		FROM profiles
		WHERE user_id = $1
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, userID)

	var profile domain.Profile
	var socialsJSON sql.NullString

	err := row.Scan(
		&profile.UserID,
		&profile.Username,
		&profile.DisplayName,
		&profile.AvatarURL,
		&profile.BannerURL,
		&profile.Bio,
		&profile.Locale,
		&profile.Timezone,
		&socialsJSON,
		&profile.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrProfileNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	if socialsJSON.Valid {
		profile.SocialsJSON = socialsJSON.String
	}

	return &profile, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, profile *domain.Profile) error {
	query := `
		UPDATE profiles
		SET username = $2, display_name = $3, avatar_url = $4, banner_url = $5, 
		    bio = $6, locale = $7, timezone = $8, socials_json = $9, updated_at = $10
		WHERE user_id = $1
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query,
		profile.UserID,
		profile.Username,
		profile.DisplayName,
		profile.AvatarURL,
		profile.BannerURL,
		profile.Bio,
		profile.Locale,
		profile.Timezone,
		profile.SocialsJSON,
		profile.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Try to create profile if it doesn't exist
		return r.CreateProfile(ctx, profile)
	}

	return nil
}
