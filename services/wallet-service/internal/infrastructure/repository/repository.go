package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
)

type Repository struct {
	postgres *postgres.Postgres
}

func NewWalletRepository(postgres *postgres.Postgres) domain.WalletRepository {
	return &Repository{postgres: postgres}
}

func (r *Repository) CreateLink(ctx context.Context, link *domain.WalletLink) error {
	query := `
		INSERT INTO wallet_links (wallet_id, user_id, account_id, address, chain_id, is_primary, type, connector, label, verified_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err := r.postgres.GetClient().ExecContext(ctx, query,
		link.ID,
		link.UserID,
		link.AccountID,
		link.Address,
		link.ChainID,
		link.IsPrimary,
		link.Type,
		link.Connector,
		link.Label,
		link.VerifiedAt,
		link.CreatedAt,
		link.UpdatedAt,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return fmt.Errorf("wallet link already exists: %w", err)
		}
		return fmt.Errorf("failed to create wallet link: %w", err)
	}

	return nil
}

func (r *Repository) GetLink(ctx context.Context, walletID domain.WalletID) (*domain.WalletLink, error) {
	query := `
		SELECT wallet_id, user_id, account_id, address, chain_id, is_primary, type, connector, label, verified_at, created_at, updated_at
		FROM wallet_links
		WHERE wallet_id = $1
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, walletID)

	var link domain.WalletLink
	var walletType, connector, label sql.NullString

	err := row.Scan(
		&link.ID,
		&link.UserID,
		&link.AccountID,
		&link.Address,
		&link.ChainID,
		&link.IsPrimary,
		&walletType,
		&connector,
		&label,
		&link.VerifiedAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet link: %w", err)
	}

	if walletType.Valid {
		link.Type = domain.WalletType(walletType.String)
	}
	if connector.Valid {
		link.Connector = connector.String
	}
	if label.Valid {
		link.Label = label.String
	}

	return &link, nil
}

func (r *Repository) GetLinkByUserAndAddress(ctx context.Context, userID domain.UserID, address domain.Address) (*domain.WalletLink, error) {
	query := `
		SELECT wallet_id, user_id, account_id, address, chain_id, is_primary, type, connector, label, verified_at, created_at, updated_at
		FROM wallet_links
		WHERE user_id = $1 AND LOWER(address) = LOWER($2)
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, userID, address)

	var link domain.WalletLink
	var walletType, connector, label sql.NullString

	err := row.Scan(
		&link.ID,
		&link.UserID,
		&link.AccountID,
		&link.Address,
		&link.ChainID,
		&link.IsPrimary,
		&walletType,
		&connector,
		&label,
		&link.VerifiedAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet link by user and address: %w", err)
	}

	if walletType.Valid {
		link.Type = domain.WalletType(walletType.String)
	}
	if connector.Valid {
		link.Connector = connector.String
	}
	if label.Valid {
		link.Label = label.String
	}

	return &link, nil
}

func (r *Repository) GetUserWallets(ctx context.Context, userID domain.UserID) ([]*domain.WalletLink, error) {
	query := `
		SELECT wallet_id, user_id, account_id, address, chain_id, is_primary, type, connector, label, verified_at, created_at, updated_at
		FROM wallet_links
		WHERE user_id = $1
		ORDER BY is_primary DESC, created_at ASC
	`

	rows, err := r.postgres.GetClient().QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user wallets: %w", err)
	}
	defer rows.Close()

	var wallets []*domain.WalletLink
	for rows.Next() {
		var link domain.WalletLink
		var walletType, connector, label sql.NullString

		err := rows.Scan(
			&link.ID,
			&link.UserID,
			&link.AccountID,
			&link.Address,
			&link.ChainID,
			&link.IsPrimary,
			&walletType,
			&connector,
			&label,
			&link.VerifiedAt,
			&link.CreatedAt,
			&link.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan wallet link: %w", err)
		}

		if walletType.Valid {
			link.Type = domain.WalletType(walletType.String)
		}
		if connector.Valid {
			link.Connector = connector.String
		}
		if label.Valid {
			link.Label = label.String
		}

		wallets = append(wallets, &link)
	}

	return wallets, nil
}

func (r *Repository) GetWalletByAddress(ctx context.Context, address domain.Address) (*domain.WalletLink, error) {
	query := `
		SELECT wallet_id, user_id, account_id, address, chain_id, is_primary, type, connector, label, verified_at, created_at, updated_at
		FROM wallet_links
		WHERE LOWER(address) = LOWER($1)
		LIMIT 1
	`

	row := r.postgres.GetClient().QueryRowContext(ctx, query, address)

	var link domain.WalletLink
	var walletType, connector, label sql.NullString

	err := row.Scan(
		&link.ID,
		&link.UserID,
		&link.AccountID,
		&link.Address,
		&link.ChainID,
		&link.IsPrimary,
		&walletType,
		&connector,
		&label,
		&link.VerifiedAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrWalletNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet by address: %w", err)
	}

	if walletType.Valid {
		link.Type = domain.WalletType(walletType.String)
	}
	if connector.Valid {
		link.Connector = connector.String
	}
	if label.Valid {
		link.Label = label.String
	}

	return &link, nil
}

func (r *Repository) UpdateLink(ctx context.Context, link *domain.WalletLink) error {
	query := `
		UPDATE wallet_links
		SET type = $2, connector = $3, label = $4, updated_at = $5
		WHERE wallet_id = $1
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query,
		link.ID,
		link.Type,
		link.Connector,
		link.Label,
		link.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update wallet link: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrWalletNotFound
	}

	return nil
}

func (r *Repository) UpdatePrimaryWallet(ctx context.Context, userID domain.UserID, walletID domain.WalletID) error {
	// Start a transaction
	tx, err := r.postgres.GetClient().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// First, set all user wallets to non-primary
	query1 := `
		UPDATE wallet_links
		SET is_primary = false
		WHERE user_id = $1
	`
	_, err = tx.ExecContext(ctx, query1, userID)
	if err != nil {
		return fmt.Errorf("failed to update wallets: %w", err)
	}

	// Then, set the specified wallet as primary
	query2 := `
		UPDATE wallet_links
		SET is_primary = true
		WHERE wallet_id = $1 AND user_id = $2
	`
	result, err := tx.ExecContext(ctx, query2, walletID, userID)
	if err != nil {
		return fmt.Errorf("failed to set primary wallet: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrWalletNotFound
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *Repository) DeleteLink(ctx context.Context, walletID domain.WalletID) error {
	query := `
		DELETE FROM wallet_links
		WHERE wallet_id = $1
	`

	result, err := r.postgres.GetClient().ExecContext(ctx, query, walletID)
	if err != nil {
		return fmt.Errorf("failed to delete wallet link: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrWalletNotFound
	}

	return nil
}
