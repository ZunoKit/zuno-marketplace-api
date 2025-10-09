package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type Repository struct {
	postgres *postgres.Postgres
	redis    *redis.Redis
}

func NewAuthRepository(postgres *postgres.Postgres, redis *redis.Redis) domain.AuthRepository {
	return &Repository{postgres: postgres, redis: redis}
}

// Nonce operations

func (r *Repository) CreateNonce(ctx context.Context, nonce *domain.Nonce) error {
	query := `
		INSERT INTO auth_nonces (nonce, account_id, domain, chain_id, issued_at, expires_at, used, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.postgres.GetClient().ExecContext(ctx, query,
		nonce.Value,
		nonce.AccountID,
		nonce.Domain,
		nonce.ChainID,
		nonce.CreatedAt,
		nonce.ExpiresAt,
		nonce.Used,
		nonce.CreatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("nonce already exists: %w", err)
		}
		return fmt.Errorf("failed to create nonce: %w", err)
	}

	// Cache nonce in Redis with TTL
	cacheKey := fmt.Sprintf("siwe:nonce:%s", nonce.Value)
	ttl := time.Until(nonce.ExpiresAt)
	if ttl > 0 {
		err = r.redis.GetClient().Set(ctx, cacheKey, "1", ttl).Err()
		if err != nil {
			// Log warning but don't fail the operation
			// In production, you might want to log this error
		}
	}

	return nil
}

func (r *Repository) GetNonce(ctx context.Context, nonceValue string) (*domain.Nonce, error) {
	// First check Redis cache
	cacheKey := fmt.Sprintf("siwe:nonce:%s", nonceValue)
	exists, err := r.redis.GetClient().Exists(ctx, cacheKey).Result()
	if err == nil && exists == 0 {
		return nil, domain.ErrNonceNotFound
	}

	query := `
		SELECT nonce, account_id, domain, chain_id, issued_at, expires_at, used, created_at
		FROM auth_nonces
		WHERE nonce = $1
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, nonceValue)

	var nonce domain.Nonce
	err = row.Scan(
		&nonce.Value,
		&nonce.AccountID,
		&nonce.Domain,
		&nonce.ChainID,
		&nonce.CreatedAt,
		&nonce.ExpiresAt,
		&nonce.Used,
		&nonce.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrNonceNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	return &nonce, nil
}

func (r *Repository) TryUseNonce(ctx context.Context, nonceValue, accountID, chainID, domain string, usedAt time.Time) (bool, error) {
	// Use the stored procedure for atomic CAS operation
	query := `SELECT try_use_nonce($1, $2, $3, $4)`

	var success bool
	err := r.postgres.GetClient().QueryRowContext(ctx, query, nonceValue, accountID, chainID, domain).Scan(&success)
	if err != nil {
		return false, fmt.Errorf("failed to use nonce: %w", err)
	}

	if success {
		// Remove from Redis cache
		cacheKey := fmt.Sprintf("siwe:nonce:%s", nonceValue)
		r.redis.GetClient().Del(ctx, cacheKey)
	}

	return success, nil
}

// Session operations

func (r *Repository) CreateSession(ctx context.Context, session *domain.Session) error {
	query := `
		INSERT INTO sessions (
			session_id, user_id, device_id, refresh_hash, previous_refresh_hash,
			token_family_id, token_generation, ip_address, user_agent, 
			created_at, expires_at, last_used_at, collection_intent_context
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := r.postgres.GetClient().ExecContext(ctx, query,
		session.ID,
		session.UserID,
		session.DeviceID,
		session.RefreshHash,
		session.PreviousRefreshHash,
		session.TokenFamilyID,
		session.TokenGeneration,
		session.IP,
		session.UA,
		session.CreatedAt,
		session.ExpiresAt,
		session.LastUsedAt,
		session.CollectionIntentContext,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

func (r *Repository) GetSession(ctx context.Context, sessionID domain.SessionID) (*domain.Session, error) {
	query := `
		SELECT session_id, user_id, device_id, refresh_hash, previous_refresh_hash,
		       token_family_id, token_generation, ip_address, user_agent, 
		       created_at, expires_at, revoked_at, revoked_reason, last_used_at
		FROM sessions
		WHERE session_id = $1 AND revoked_at IS NULL
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, sessionID)

	var session domain.Session
	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceID,
		&session.RefreshHash,
		&session.PreviousRefreshHash,
		&session.TokenFamilyID,
		&session.TokenGeneration,
		&session.IP,
		&session.UA,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.RevokedReason,
		&session.LastUsedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

func (r *Repository) GetSessionByRefreshHash(ctx context.Context, refreshHash string) (*domain.Session, error) {
	query := `
		SELECT session_id, user_id, device_id, refresh_hash, previous_refresh_hash,
		       token_family_id, token_generation, ip_address, user_agent,
		       created_at, expires_at, revoked_at, revoked_reason, last_used_at
		FROM sessions
		WHERE refresh_hash = $1 AND revoked_at IS NULL AND expires_at > now()
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, refreshHash)

	var session domain.Session
	err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.DeviceID,
		&session.RefreshHash,
		&session.PreviousRefreshHash,
		&session.TokenFamilyID,
		&session.TokenGeneration,
		&session.IP,
		&session.UA,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.RevokedReason,
		&session.LastUsedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found or expired")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by refresh hash: %w", err)
	}

	return &session, nil
}

func (r *Repository) UpdateSessionLastUsed(ctx context.Context, sessionID domain.SessionID) error {
	query := `
		UPDATE sessions 
		SET last_used_at = now() 
		WHERE session_id = $1 AND revoked_at IS NULL
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to update session last used: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found or already revoked")
	}

	return nil
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID domain.SessionID) error {
	query := `
		UPDATE sessions 
		SET revoked_at = now() 
		WHERE session_id = $1 AND revoked_at IS NULL
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found or already revoked")
	}

	return nil
}

// RevokeSessionWithReason revokes a session with a specific reason
func (r *Repository) RevokeSessionWithReason(ctx context.Context, sessionID domain.SessionID, reason string) error {
	query := `
		UPDATE sessions 
		SET revoked_at = now(), revoked_reason = $2
		WHERE session_id = $1 AND revoked_at IS NULL
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query, sessionID, reason)
	if err != nil {
		return fmt.Errorf("failed to revoke session with reason: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found or already revoked")
	}

	return nil
}

// RotateRefreshToken updates the refresh token for a session (token rotation)
func (r *Repository) RotateRefreshToken(ctx context.Context, sessionID domain.SessionID, newRefreshHash string) error {
	query := `
		UPDATE sessions 
		SET previous_refresh_hash = refresh_hash,
		    refresh_hash = $2,
		    token_generation = token_generation + 1,
		    last_used_at = now()
		WHERE session_id = $1 AND revoked_at IS NULL
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query, sessionID, newRefreshHash)
	if err != nil {
		return fmt.Errorf("failed to rotate refresh token: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found or already revoked")
	}

	return nil
}

// GetSessionsByTokenFamily gets all sessions in a token family
func (r *Repository) GetSessionsByTokenFamily(ctx context.Context, tokenFamilyID string) ([]*domain.Session, error) {
	query := `
		SELECT session_id, user_id, device_id, refresh_hash, previous_refresh_hash,
		       token_family_id, token_generation, ip_address, user_agent,
		       created_at, expires_at, revoked_at, revoked_reason, last_used_at
		FROM sessions
		WHERE token_family_id = $1
		ORDER BY token_generation DESC
	`

	rows, err := r.postgres.GetClient().QueryContext(ctx, query, tokenFamilyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sessions by token family: %w", err)
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		var session domain.Session
		err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.DeviceID,
			&session.RefreshHash,
			&session.PreviousRefreshHash,
			&session.TokenFamilyID,
			&session.TokenGeneration,
			&session.IP,
			&session.UA,
			&session.CreatedAt,
			&session.ExpiresAt,
			&session.RevokedAt,
			&session.RevokedReason,
			&session.LastUsedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// RevokeTokenFamily revokes all sessions in a token family
func (r *Repository) RevokeTokenFamily(ctx context.Context, tokenFamilyID string, reason string) error {
	query := `
		UPDATE sessions 
		SET revoked_at = now(), revoked_reason = $2
		WHERE token_family_id = $1 AND revoked_at IS NULL
	`

	_, err := r.postgres.GetClient().ExecContext(ctx, query, tokenFamilyID, reason)
	if err != nil {
		return fmt.Errorf("failed to revoke token family: %w", err)
	}

	return nil
}

// CheckTokenReuse checks if a refresh token hash has been used before (found in previous_refresh_hash)
func (r *Repository) CheckTokenReuse(ctx context.Context, refreshHash string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM sessions 
			WHERE previous_refresh_hash = $1
		)
	`

	var exists bool
	err := r.postgres.GetClient().QueryRowContext(ctx, query, refreshHash).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check token reuse: %w", err)
	}

	return exists, nil
}
