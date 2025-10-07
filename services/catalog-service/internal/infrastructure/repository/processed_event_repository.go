package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type ProcessedEventRepository struct {
	postgresDb *postgres.Postgres
	redisDb    *redis.Redis
}

// NewProcessedEventRepository creates a new PostgreSQL processed event repository
func NewProcessedEventRepository(postgresDb *postgres.Postgres, redisDb *redis.Redis) domain.ProcessedEventsRepository {
	return &ProcessedEventRepository{
		postgresDb: postgresDb,
		redisDb:    redisDb,
	}
}

func (r *ProcessedEventRepository) MarkProcessed(ctx context.Context, eventID string) (bool, error) {
	// Check if already processed using cache first
	cacheKey := fmt.Sprintf("processed_event:%s", eventID)
	exists, err := r.redisDb.Exists(ctx, cacheKey)
	if err == nil && exists > 0 {
		return false, nil // Already processed
	}

	// Try to insert into database
	query := `
		INSERT INTO processed_events (event_id, event_version, processed_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING
	`

	result, err := r.postgresDb.GetClient().ExecContext(ctx, query, eventID, 1, time.Now())
	if err != nil {
		return false, fmt.Errorf("failed to mark event as processed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected > 0 {
		// Event was newly processed, cache it
		r.redisDb.SetWithExpiration(ctx, cacheKey, "processed", 24*time.Hour)
		return true, nil
	}

	// Event was already processed
	return false, nil
}

// GetProcessedEvent retrieves a processed event by ID
func (r *ProcessedEventRepository) GetProcessedEvent(ctx context.Context, eventID string) (*domain.ProcessedEvent, error) {
	query := `
		SELECT event_id, event_version, chain_id, block_hash, log_index, processed_at
		FROM processed_events
		WHERE event_id = $1
	`

	var processedEvent domain.ProcessedEvent
	var chainID, blockHash sql.NullString
	var logIndex sql.NullInt32

	err := r.postgresDb.GetClient().QueryRowContext(ctx, query, eventID).Scan(
		&processedEvent.EventID,
		&processedEvent.EventType, // This will be empty as we don't store it
		&chainID,
		&blockHash,
		&logIndex,
		&processedEvent.ProcessedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get processed event: %w", err)
	}

	if chainID.Valid {
		processedEvent.ChainID = chainID.String
	}
	if blockHash.Valid {
		processedEvent.TxHash = blockHash.String
	}

	return &processedEvent, nil
}

// IsProcessed checks if an event has been processed
func (r *ProcessedEventRepository) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("processed_event:%s", eventID)
	exists, err := r.redisDb.Exists(ctx, cacheKey)
	if err == nil && exists > 0 {
		return true, nil
	}

	// Check database
	query := `SELECT 1 FROM processed_events WHERE event_id = $1 LIMIT 1`

	var existsFlag int
	err = r.postgresDb.GetClient().QueryRowContext(ctx, query, eventID).Scan(&existsFlag)

	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check if event is processed: %w", err)
	}

	return true, nil
}
