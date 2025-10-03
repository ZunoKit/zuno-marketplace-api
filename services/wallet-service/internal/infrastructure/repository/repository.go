package repository

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type Repository struct {
	postgres *postgres.Postgres
	redis    *redis.Redis
}

type txRepo struct {
	tx *sql.Tx
}

func NewWalletRepository(pg *postgres.Postgres, rds *redis.Redis) domain.WalletRepository {
	return &Repository{
		postgres: pg,
		redis:    rds,
	}
}

func (r *Repository) WithTx(ctx context.Context, fn func(domain.TxWalletRepository) error) error {
    if r.postgres == nil || r.postgres.GetClient() == nil {
        return fmt.Errorf("database operation unavailable: postgres client is nil")
    }
    tx, err := r.postgres.GetClient().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	txRepo := &txRepo{tx: tx}

	if err := fn(txRepo); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction error: %v, rollback error: %w", err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}

func (r *txRepo) AcquireAccountLock(ctx context.Context, accountID string) error {
	// Use advisory lock to prevent concurrent operations on same account
	lockKey := fmt.Sprintf("account_%s", accountID)
	hashKey := hashString(lockKey)

	_, err := r.tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", hashKey)
	if err != nil {
		return fmt.Errorf("failed to acquire account lock: %w", err)
	}
	return nil
}

func (r *txRepo) AcquireAddressLock(ctx context.Context, chainID, address string) error {
	// Use advisory lock to prevent concurrent operations on same address
	lockKey := fmt.Sprintf("address_%s_%s", chainID, address)
	hashKey := hashString(lockKey)

	_, err := r.tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock($1)", hashKey)
	if err != nil {
		return fmt.Errorf("failed to acquire address lock: %w", err)
	}
	return nil
}

func (r *txRepo) GetByAccountIDTx(ctx context.Context, accountID string) (*domain.WalletLink, error) {
	query := `
		SELECT id, user_id, account_id, address, chain_id, is_primary, 
		       verified_at, created_at, updated_at
		FROM wallets 
		WHERE account_id = $1
		LIMIT 1`

	var link domain.WalletLink
	var verifiedAt sql.NullTime

	err := r.tx.QueryRowContext(ctx, query, accountID).Scan(
		&link.ID, &link.UserID, &link.AccountID, &link.Address, &link.ChainID,
		&link.IsPrimary, &verifiedAt, &link.CreatedAt, &link.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("failed to get wallet by account ID: %w", err)
	}

	if verifiedAt.Valid {
		link.VerifiedAt = &verifiedAt.Time
	}

	return &link, nil
}

func (r *txRepo) GetByAddressTx(ctx context.Context, chainID, address string) (*domain.WalletLink, error) {
	query := `
		SELECT id, user_id, account_id, address, chain_id, is_primary, 
		       verified_at, created_at, updated_at
		FROM wallets 
		WHERE chain_id = $1 AND address = $2
		LIMIT 1`

	var link domain.WalletLink
	var verifiedAt sql.NullTime

	err := r.tx.QueryRowContext(ctx, query, chainID, address).Scan(
		&link.ID, &link.UserID, &link.AccountID, &link.Address, &link.ChainID,
		&link.IsPrimary, &verifiedAt, &link.CreatedAt, &link.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("failed to get wallet by address: %w", err)
	}

	if verifiedAt.Valid {
		link.VerifiedAt = &verifiedAt.Time
	}

	return &link, nil
}

func (r *txRepo) InsertWalletTx(ctx context.Context, link domain.WalletLink) (*domain.WalletLink, error) {
	// Generate new ID if not provided
	if link.ID == "" {
		link.ID = uuid.New().String()
	}

	now := time.Now()
	link.CreatedAt = now
	link.UpdatedAt = now

	// Set verified_at to now if this is being created
	if link.VerifiedAt == nil {
		link.VerifiedAt = &now
	}

	query := `
		INSERT INTO wallets (id, user_id, account_id, address, chain_id, is_primary, 
		                    verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, user_id, account_id, address, chain_id, is_primary, 
		          verified_at, created_at, updated_at`

	var result domain.WalletLink
	var verifiedAt sql.NullTime

	err := r.tx.QueryRowContext(ctx, query,
		link.ID, link.UserID, link.AccountID, link.Address, link.ChainID,
		link.IsPrimary, link.VerifiedAt, link.CreatedAt, link.UpdatedAt,
	).Scan(
		&result.ID, &result.UserID, &result.AccountID, &result.Address, &result.ChainID,
		&result.IsPrimary, &verifiedAt, &result.CreatedAt, &result.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to insert wallet: %w", err)
	}

	if verifiedAt.Valid {
		result.VerifiedAt = &verifiedAt.Time
	}

	return &result, nil
}

func (r *txRepo) UpdateWalletMetaTx(ctx context.Context, id domain.WalletID, isPrimary *bool, verifiedAt *time.Time, lastSeen *time.Time, label *string) (*domain.WalletLink, error) {
	// Build dynamic update query
	setParts := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argIndex := 1

	if isPrimary != nil {
		setParts = append(setParts, fmt.Sprintf("is_primary = $%d", argIndex))
		args = append(args, *isPrimary)
		argIndex++
	}

	if verifiedAt != nil {
		setParts = append(setParts, fmt.Sprintf("verified_at = $%d", argIndex))
		args = append(args, *verifiedAt)
		argIndex++
	}

	if lastSeen != nil {
		setParts = append(setParts, fmt.Sprintf("last_seen_at = $%d", argIndex))
		args = append(args, *lastSeen)
		argIndex++
	}

	if label != nil {
		setParts = append(setParts, fmt.Sprintf("label = $%d", argIndex))
		args = append(args, *label)
		argIndex++
	}

	// Add ID to args
	args = append(args, id)

	// Build SET clause
	setClause := ""
	for i, part := range setParts {
		if i > 0 {
			setClause += ", "
		}
		setClause += part
	}

	query := fmt.Sprintf(`
		UPDATE wallets 
		SET %s
		WHERE id = $%d
		RETURNING id, user_id, account_id, address, chain_id, is_primary, 
		          verified_at, created_at, updated_at`,
		setClause, argIndex)

	var result domain.WalletLink
	var verifiedAtResult sql.NullTime

	err := r.tx.QueryRowContext(ctx, query, args...).Scan(
		&result.ID, &result.UserID, &result.AccountID, &result.Address, &result.ChainID,
		&result.IsPrimary, &verifiedAtResult, &result.CreatedAt, &result.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("failed to update wallet: %w", err)
	}

	if verifiedAtResult.Valid {
		result.VerifiedAt = &verifiedAtResult.Time
	}

	return &result, nil
}

func (r *txRepo) GetPrimaryByUserTx(ctx context.Context, userID domain.UserID) (*domain.WalletLink, error) {
	query := `
		SELECT id, user_id, account_id, address, chain_id, is_primary, 
		       verified_at, created_at, updated_at
		FROM wallets 
		WHERE user_id = $1 AND is_primary = true
		LIMIT 1`

	var link domain.WalletLink
	var verifiedAt sql.NullTime

	err := r.tx.QueryRowContext(ctx, query, userID).Scan(
		&link.ID, &link.UserID, &link.AccountID, &link.Address, &link.ChainID,
		&link.IsPrimary, &verifiedAt, &link.CreatedAt, &link.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("failed to get primary wallet: %w", err)
	}

	if verifiedAt.Valid {
		link.VerifiedAt = &verifiedAt.Time
	}

	return &link, nil
}

func (r *txRepo) DemoteOtherPrimariesTx(ctx context.Context, userID domain.UserID, chainID domain.ChainID, keepID domain.WalletID) error {
	const q = `UPDATE wallets SET is_primary=false, updated_at=now()
               WHERE user_id=$1 AND chain_id=$2 AND is_primary=true AND ($3='' OR id<>$3)`
	_, err := r.tx.ExecContext(ctx, q, userID, chainID, keepID)
	if err != nil {
		return fmt.Errorf("demote primaries: %w", err)
	}
	return nil
}

func (r *txRepo) UpdateWalletAddressTx(ctx context.Context, id domain.WalletID, chainID domain.ChainID, address domain.Address) (*domain.WalletLink, error) {
	const q = `
UPDATE wallets SET chain_id=$2, address=$3, updated_at=now(), verified_at=COALESCE(verified_at, now())
WHERE id=$1
RETURNING id,user_id,account_id,address,chain_id,is_primary,verified_at,created_at,updated_at`
	var out domain.WalletLink
	var ver sql.NullTime
	err := r.tx.QueryRowContext(ctx, q, id, chainID, address).Scan(
		&out.ID, &out.UserID, &out.AccountID, &out.Address, &out.ChainID, &out.IsPrimary, &ver, &out.CreatedAt, &out.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update address: %w", err)
	}
	if ver.Valid {
		out.VerifiedAt = &ver.Time
	}
	return &out, nil
}

func (r *txRepo) GetPrimaryByUserChainTx(ctx context.Context, userID domain.UserID, chainID domain.ChainID) (*domain.WalletLink, error) {
	const q = `SELECT id,user_id,account_id,address,chain_id,is_primary,verified_at,created_at,updated_at
               FROM wallets WHERE user_id=$1 AND chain_id=$2 AND is_primary=true LIMIT 1`
	var out domain.WalletLink
	var ver sql.NullTime
	err := r.tx.QueryRowContext(ctx, q, userID, chainID).Scan(
		&out.ID, &out.UserID, &out.AccountID, &out.Address, &out.ChainID,
		&out.IsPrimary, &ver, &out.CreatedAt, &out.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrWalletNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get primary wallet by chain: %w", err)
	}

	if ver.Valid {
		out.VerifiedAt = &ver.Time
	}
	return &out, nil
}

// Helper function to hash strings for advisory locks
func hashString(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}

// HashString exposes the internal hashing helper for tests/benchmarks
func HashString(s string) int64 { return hashString(s) }
