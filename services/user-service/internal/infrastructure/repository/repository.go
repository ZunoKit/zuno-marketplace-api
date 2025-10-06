package repository

import (
	"context"
	"database/sql"
	"hash/fnv"
	"strings"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type Repository struct {
	db    *postgres.Postgres
	redis *redis.Redis
}

type txRepo struct{ tx *sql.Tx }

func NewUserRepository(db *postgres.Postgres, redis *redis.Redis) domain.UserRepository {
	return &Repository{db: db, redis: redis}
}

func (r *Repository) GetUserIDByAccount(ctx context.Context, accountID string) (userID string, err error) {
	query := `SELECT user_id FROM user_accounts WHERE account_id = $1 LIMIT 1`

	err = r.db.GetClient().QueryRowContext(ctx, query, accountID).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", domain.ErrUserNotFound
		}
		return "", domain.NewDatabaseError("get_user_by_account", err)
	}

	return userID, nil
}

func (r *Repository) WithTx(ctx context.Context, fn func(domain.TxUserRepository) error) error {
	tx, err := r.db.GetClient().BeginTx(ctx, nil)
	if err != nil {
		return domain.NewDatabaseError("begin_tx", err)
	}
	w := &txRepo{tx: tx}
	if err := fn(w); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return domain.NewDatabaseError("commit_tx", err)
	}
	return nil
}

func advisoryKey(s string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int64(h.Sum64())
}

func (t *txRepo) AcquireAccountLock(ctx context.Context, accountID string) error {
	const q = `SELECT pg_advisory_xact_lock($1)`
	if _, err := t.tx.ExecContext(ctx, q, advisoryKey(accountID)); err != nil {
		return domain.NewDatabaseError("advisory_lock", err)
	}
	return nil
}

func (t *txRepo) GetUserIDByAccountTx(ctx context.Context, accountID string) (string, error) {
	const q = `SELECT user_id FROM user_accounts WHERE account_id = $1 LIMIT 1`
	var uid string
	err := t.tx.QueryRowContext(ctx, q, accountID).Scan(&uid)
	if err == sql.ErrNoRows {
		return "", domain.ErrUserNotFound
	}
	if err != nil {
		return "", domain.NewDatabaseError("get_user_by_account_tx", err)
	}
	return uid, nil
}

func (t *txRepo) CreateUserTx(ctx context.Context) (string, error) {
	const q = `INSERT INTO users(status, created_at) VALUES('active', now()) RETURNING id`
	var uid string
	if err := t.tx.QueryRowContext(ctx, q).Scan(&uid); err != nil {
		return "", domain.NewDatabaseError("create_user_tx", err)
	}
	return uid, nil
}

func (t *txRepo) CreateProfileTx(ctx context.Context, userID string) error {
	const q = `INSERT INTO profiles (user_id, locale, timezone, socials_json, updated_at)
	           VALUES ($1, $2, $3, '{}', now())
	           ON CONFLICT (user_id) DO NOTHING`
	_, err := t.tx.ExecContext(ctx, q, userID, domain.DefaultLocale, domain.DefaultTimezone)
	if err != nil {
		return domain.NewDatabaseError("create_profile_tx", err)
	}
	return nil
}

func (t *txRepo) UpsertUserAccountTx(ctx context.Context, userID, accountID, address, chainID string) error {
	const q = `
INSERT INTO user_accounts (account_id, user_id, address, chain_id, created_at, last_seen_at)
VALUES ($1, $2, $3, $4, now(), now())
ON CONFLICT (account_id)
DO UPDATE SET
  address     = COALESCE(EXCLUDED.address, user_accounts.address),
  chain_id    = COALESCE(EXCLUDED.chain_id, user_accounts.chain_id),
  last_seen_at= now()
`
	addr := strings.ToLower(address)
	_, err := t.tx.ExecContext(ctx, q, accountID, userID, addr, chainID)
	if err != nil {
		return domain.NewDatabaseError("upsert_user_account_tx", err)
	}
	return nil
}

func (t *txRepo) TouchUserAccountTx(ctx context.Context, accountID, address string) error {
	const q = `
UPDATE user_accounts
SET last_seen_at = now(),
    address = COALESCE($2, address)
WHERE account_id = $1`
	_, err := t.tx.ExecContext(ctx, q, accountID, strings.ToLower(address))
	if err != nil {
		return domain.NewDatabaseError("touch_user_account_tx", err)
	}
	return nil
}
