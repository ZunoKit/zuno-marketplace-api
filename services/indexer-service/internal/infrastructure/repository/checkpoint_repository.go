package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
)

type CheckpointRepository struct {
	db *postgres.Postgres
}

// NewCheckpointRepository creates a new PostgreSQL checkpoint repository
func NewCheckpointRepository(client *postgres.Postgres) *CheckpointRepository {
	repo := &CheckpointRepository{
		db: client,
	}

	// Initialize database schema
	if err := repo.initSchema(); err != nil {
		fmt.Printf("Warning: failed to initialize checkpoint schema: %v\n", err)
	}

	return repo
}

// initSchema creates the indexer_checkpoints table if it doesn't exist
func (r *CheckpointRepository) initSchema() error {
	ctx := context.Background()

	createTableQuery := `
		CREATE TABLE IF NOT EXISTS indexer_checkpoints (
			chain_id VARCHAR(50) PRIMARY KEY,
			last_block NUMERIC(78, 0) NOT NULL DEFAULT 0,
			last_block_hash VARCHAR(66) NOT NULL DEFAULT '',
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		
		-- Create index for faster lookups
		CREATE INDEX IF NOT EXISTS idx_checkpoints_updated_at ON indexer_checkpoints(updated_at);
	`

	_, err := r.db.GetClient().ExecContext(ctx, createTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create indexer_checkpoints table: %w", err)
	}

	return nil
}

// GetCheckpoint retrieves the latest checkpoint for a chain
func (r *CheckpointRepository) GetCheckpoint(ctx context.Context, chainID string) (*domain.Checkpoint, error) {
	query := `
		SELECT chain_id, last_block, last_block_hash, updated_at
		FROM indexer_checkpoints
		WHERE chain_id = $1
	`

	var checkpoint domain.Checkpoint
	var lastBlockStr string

	err := r.db.GetClient().QueryRowContext(ctx, query, chainID).Scan(
		&checkpoint.ChainID,
		&lastBlockStr,
		&checkpoint.LastBlockHash,
		&checkpoint.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Return default checkpoint starting from block 0
			return &domain.Checkpoint{
				ChainID:       chainID,
				LastBlock:     big.NewInt(0),
				LastBlockHash: "",
				UpdatedAt:     time.Now(),
			}, nil
		}
		return nil, fmt.Errorf("failed to get checkpoint for chain %s: %w", chainID, err)
	}

	// Convert string to big.Int
	lastBlock := new(big.Int)
	if lastBlockStr != "" {
		if _, ok := lastBlock.SetString(lastBlockStr, 10); !ok {
			return nil, fmt.Errorf("invalid block number format: %s", lastBlockStr)
		}
	}
	checkpoint.LastBlock = lastBlock

	return &checkpoint, nil
}

// UpdateCheckpoint updates the checkpoint for a chain
func (r *CheckpointRepository) UpdateCheckpoint(ctx context.Context, checkpoint *domain.Checkpoint) error {
	if checkpoint == nil {
		return fmt.Errorf("checkpoint cannot be nil")
	}

	query := `
		UPDATE indexer_checkpoints
		SET last_block = $2,
			last_block_hash = $3,
			updated_at = $4
		WHERE chain_id = $1
	`

	lastBlockStr := checkpoint.LastBlock.String()
	updatedAt := time.Now()

	result, err := r.db.GetClient().ExecContext(ctx, query,
		checkpoint.ChainID,
		lastBlockStr,
		checkpoint.LastBlockHash,
		updatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update checkpoint for chain %s: %w", checkpoint.ChainID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		// Checkpoint doesn't exist, create it
		return r.CreateCheckpoint(ctx, checkpoint)
	}

	// Update the checkpoint timestamp
	checkpoint.UpdatedAt = updatedAt

	return nil
}

// CreateCheckpoint creates a new checkpoint for a chain
func (r *CheckpointRepository) CreateCheckpoint(ctx context.Context, checkpoint *domain.Checkpoint) error {
	if checkpoint == nil {
		return fmt.Errorf("checkpoint cannot be nil")
	}

	query := `
		INSERT INTO indexer_checkpoints (chain_id, last_block, last_block_hash, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chain_id) DO UPDATE SET
			last_block = EXCLUDED.last_block,
			last_block_hash = EXCLUDED.last_block_hash,
			updated_at = EXCLUDED.updated_at
	`

	lastBlockStr := checkpoint.LastBlock.String()
	updatedAt := time.Now()

	_, err := r.db.GetClient().ExecContext(ctx, query,
		checkpoint.ChainID,
		lastBlockStr,
		checkpoint.LastBlockHash,
		updatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create checkpoint for chain %s: %w", checkpoint.ChainID, err)
	}

	// Update the checkpoint timestamp
	checkpoint.UpdatedAt = updatedAt

	return nil
}

// GetAllCheckpoints retrieves all checkpoints
func (r *CheckpointRepository) GetAllCheckpoints(ctx context.Context) ([]*domain.Checkpoint, error) {
	query := `
		SELECT chain_id, last_block, last_block_hash, updated_at
		FROM indexer_checkpoints
		ORDER BY updated_at DESC
	`

	rows, err := r.db.GetClient().QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all checkpoints: %w", err)
	}
	defer rows.Close()

	var checkpoints []*domain.Checkpoint

	for rows.Next() {
		var checkpoint domain.Checkpoint
		var lastBlockStr string

		err := rows.Scan(
			&checkpoint.ChainID,
			&lastBlockStr,
			&checkpoint.LastBlockHash,
			&checkpoint.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan checkpoint: %w", err)
		}

		// Convert string to big.Int
		lastBlock := new(big.Int)
		if lastBlockStr != "" {
			if _, ok := lastBlock.SetString(lastBlockStr, 10); !ok {
				return nil, fmt.Errorf("invalid block number format: %s", lastBlockStr)
			}
		}
		checkpoint.LastBlock = lastBlock

		checkpoints = append(checkpoints, &checkpoint)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating checkpoints: %w", err)
	}

	return checkpoints, nil
}

// IncrementCheckpoint increments the checkpoint by one block
func (r *CheckpointRepository) IncrementCheckpoint(ctx context.Context, chainID, blockHash string) error {
	// Begin transaction for atomic update
	tx, err := r.db.GetClient().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get current checkpoint
	checkpoint, err := r.GetCheckpoint(ctx, chainID)
	if err != nil {
		return fmt.Errorf("failed to get current checkpoint: %w", err)
	}

	// Increment block number
	newBlock := new(big.Int).Add(checkpoint.LastBlock, big.NewInt(1))

	// Update checkpoint
	updatedCheckpoint := &domain.Checkpoint{
		ChainID:       chainID,
		LastBlock:     newBlock,
		LastBlockHash: blockHash,
		UpdatedAt:     time.Now(),
	}

	err = r.UpdateCheckpoint(ctx, updatedCheckpoint)
	if err != nil {
		return fmt.Errorf("failed to update checkpoint: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit checkpoint update: %w", err)
	}

	return nil
}

// SetCheckpointToBlock sets the checkpoint to a specific block
func (r *CheckpointRepository) SetCheckpointToBlock(ctx context.Context, chainID string, blockNumber *big.Int, blockHash string) error {
	checkpoint := &domain.Checkpoint{
		ChainID:       chainID,
		LastBlock:     blockNumber,
		LastBlockHash: blockHash,
		UpdatedAt:     time.Now(),
	}

	return r.UpdateCheckpoint(ctx, checkpoint)
}

// DeleteCheckpoint removes a checkpoint for a specific chain
func (r *CheckpointRepository) DeleteCheckpoint(ctx context.Context, chainID string) error {
	query := `DELETE FROM indexer_checkpoints WHERE chain_id = $1`

	result, err := r.db.GetClient().ExecContext(ctx, query, chainID)
	if err != nil {
		return fmt.Errorf("failed to delete checkpoint for chain %s: %w", chainID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("checkpoint not found for chain %s", chainID)
	}

	return nil
}

// Health check for the repository
func (r *CheckpointRepository) HealthCheck(ctx context.Context) error {
	query := `SELECT 1`
	var result int
	err := r.db.GetClient().QueryRowContext(ctx, query).Scan(&result)
	if err != nil {
		return fmt.Errorf("checkpoint repository health check failed: %w", err)
	}
	return nil
}

// GetLastProcessedBlock is a helper function to get just the block number
func (r *CheckpointRepository) GetLastProcessedBlock(ctx context.Context, chainID string) (*big.Int, error) {
	checkpoint, err := r.GetCheckpoint(ctx, chainID)
	if err != nil {
		return nil, err
	}
	return checkpoint.LastBlock, nil
}
