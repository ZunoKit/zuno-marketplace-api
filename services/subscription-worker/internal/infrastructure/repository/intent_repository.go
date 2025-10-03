package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/domain"
	sharedRedis "github.com/quangdang46/NFT-Marketplace/shared/redis"
)

const (
	// Redis key patterns
	intentStatusKeyPrefix    = "intent:status:"
	contractIntentsKeyPrefix = "intent:contract:"
	txHashIntentsKeyPrefix   = "intent:txhash:"
	expiredIntentsKey        = "intent:expired"

	// Default TTL for intent status
	defaultIntentTTL = 24 * time.Hour
)

type IntentRepository struct {
	redis *sharedRedis.Redis
}

// NewIntentRepository creates a new Redis intent repository
func NewIntentRepository(client *sharedRedis.Redis) *IntentRepository {
	return &IntentRepository{
		redis: client,
	}
}

// GetIntentStatus retrieves the status of an intent from Redis
func (r *IntentRepository) GetIntentStatus(ctx context.Context, intentID string) (*domain.IntentStatus, error) {
	key := intentStatusKeyPrefix + intentID

	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("intent not found: %s", intentID)
		}
		return nil, fmt.Errorf("failed to get intent status: %w", err)
	}

	var status domain.IntentStatus
	if err := json.Unmarshal([]byte(data), &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intent status: %w", err)
	}

	return &status, nil
}

// UpdateIntentStatus updates the status of an intent in Redis
func (r *IntentRepository) UpdateIntentStatus(ctx context.Context, status *domain.IntentStatus) error {
	if status == nil {
		return fmt.Errorf("intent status cannot be nil")
	}

	status.UpdatedAt = time.Now()

	// Serialize status to JSON
	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal intent status: %w", err)
	}

	// Store in Redis with TTL
	key := intentStatusKeyPrefix + status.IntentID
	ttl := defaultIntentTTL
	if !status.ExpiresAt.IsZero() {
		ttl = time.Until(status.ExpiresAt)
		if ttl <= 0 {
			ttl = time.Minute // Minimum TTL
		}
	}

	err = r.redis.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set intent status: %w", err)
	}

	// Update contract index if contract address is available
	if status.ContractAddress != "" && status.ChainID != "" {
		err = r.addToContractIndex(ctx, status)
		if err != nil {
			// Log error but don't fail the main operation
			fmt.Printf("Warning: failed to update contract index: %v\n", err)
		}
	}

	// Update transaction hash index if tx hash is available
	if status.TxHash != "" && status.ChainID != "" {
		err = r.addToTxHashIndex(ctx, status)
		if err != nil {
			// Log error but don't fail the main operation
			fmt.Printf("Warning: failed to update tx hash index: %v\n", err)
		}
	}

	// Add to expired set if the intent has expired
	if !status.ExpiresAt.IsZero() && time.Now().After(status.ExpiresAt) {
		err = r.redis.SAdd(ctx, expiredIntentsKey, status.IntentID).Err()
		if err != nil {
			fmt.Printf("Warning: failed to add to expired set: %v\n", err)
		}
	}

	return nil
}

// GetPendingIntentsByContract gets pending intents for a contract address
func (r *IntentRepository) GetPendingIntentsByContract(ctx context.Context, chainID, contractAddress string) ([]*domain.IntentStatus, error) {
	key := r.getContractKey(chainID, contractAddress)

	// Get all intent IDs for this contract
	intentIDs, err := r.redis.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get contract intents: %w", err)
	}

	var intents []*domain.IntentStatus
	for _, intentID := range intentIDs {
		status, err := r.GetIntentStatus(ctx, intentID)
		if err != nil {
			// Skip missing intents (might have expired)
			continue
		}

		// Only return pending/processing intents
		if status.Status == "pending" || status.Status == "processing" {
			intents = append(intents, status)
		}
	}

	return intents, nil
}

// GetIntentsByTxHash gets intents by transaction hash
func (r *IntentRepository) GetIntentsByTxHash(ctx context.Context, chainID, txHash string) ([]*domain.IntentStatus, error) {
	key := r.getTxHashKey(chainID, txHash)

	// Get all intent IDs for this transaction hash
	intentIDs, err := r.redis.SMembers(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get tx hash intents: %w", err)
	}

	var intents []*domain.IntentStatus
	for _, intentID := range intentIDs {
		status, err := r.GetIntentStatus(ctx, intentID)
		if err != nil {
			// Skip missing intents
			continue
		}
		intents = append(intents, status)
	}

	return intents, nil
}

// DeleteIntent removes an intent from Redis
func (r *IntentRepository) DeleteIntent(ctx context.Context, intentID string) error {
	// Get intent status first to clean up indexes
	status, err := r.GetIntentStatus(ctx, intentID)
	if err != nil {
		// Intent might already be deleted
		return nil
	}

	// Delete main intent key
	key := intentStatusKeyPrefix + intentID
	err = r.redis.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete intent: %w", err)
	}

	// Clean up indexes
	if status.ContractAddress != "" && status.ChainID != "" {
		contractKey := r.getContractKey(status.ChainID, status.ContractAddress)
		r.redis.SRem(ctx, contractKey, intentID)
	}

	if status.TxHash != "" && status.ChainID != "" {
		txHashKey := r.getTxHashKey(status.ChainID, status.TxHash)
		r.redis.SRem(ctx, txHashKey, intentID)
	}

	// Remove from expired set
	r.redis.SRem(ctx, expiredIntentsKey, intentID)

	return nil
}

// GetExpiredIntents gets all expired intents for cleanup
func (r *IntentRepository) GetExpiredIntents(ctx context.Context) ([]*domain.IntentStatus, error) {
	// Get expired intent IDs from the set
	expiredIDs, err := r.redis.SMembers(ctx, expiredIntentsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get expired intent IDs: %w", err)
	}

	var expiredIntents []*domain.IntentStatus
	for _, intentID := range expiredIDs {
		status, err := r.GetIntentStatus(ctx, intentID)
		if err != nil {
			// Intent might have been cleaned up already, remove from set
			r.redis.SRem(ctx, expiredIntentsKey, intentID)
			continue
		}

		// Double-check if it's actually expired
		if !status.ExpiresAt.IsZero() && time.Now().After(status.ExpiresAt) {
			expiredIntents = append(expiredIntents, status)
		} else {
			// Remove from expired set if not actually expired
			r.redis.SRem(ctx, expiredIntentsKey, intentID)
		}
	}

	return expiredIntents, nil
}

// HealthCheck performs a health check on the repository
func (r *IntentRepository) HealthCheck(ctx context.Context) error {
	return r.redis.HealthCheck(ctx)
}

// Helper methods

func (r *IntentRepository) addToContractIndex(ctx context.Context, status *domain.IntentStatus) error {
	key := r.getContractKey(status.ChainID, status.ContractAddress)
	return r.redis.SAdd(ctx, key, status.IntentID).Err()
}

func (r *IntentRepository) addToTxHashIndex(ctx context.Context, status *domain.IntentStatus) error {
	key := r.getTxHashKey(status.ChainID, status.TxHash)
	return r.redis.SAdd(ctx, key, status.IntentID).Err()
}

func (r *IntentRepository) getContractKey(chainID, contractAddress string) string {
	return fmt.Sprintf("%s%s:%s", contractIntentsKeyPrefix, chainID, strings.ToLower(contractAddress))
}

func (r *IntentRepository) getTxHashKey(chainID, txHash string) string {
	return fmt.Sprintf("%s%s:%s", txHashIntentsKeyPrefix, chainID, strings.ToLower(txHash))
}

// GetIntentsByStatus gets intents by status (for monitoring/debugging)
func (r *IntentRepository) GetIntentsByStatus(ctx context.Context, status string, limit int) ([]*domain.IntentStatus, error) {
	// This is a more expensive operation as we need to scan keys
	pattern := intentStatusKeyPrefix + "*"

	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get intent keys: %w", err)
	}

	var matchingIntents []*domain.IntentStatus
	count := 0

	for _, key := range keys {
		if limit > 0 && count >= limit {
			break
		}

		intentID := strings.TrimPrefix(key, intentStatusKeyPrefix)
		intentStatus, err := r.GetIntentStatus(ctx, intentID)
		if err != nil {
			continue
		}

		if intentStatus.Status == status {
			matchingIntents = append(matchingIntents, intentStatus)
			count++
		}
	}

	return matchingIntents, nil
}

// GetIntentStatistics returns statistics about intents
func (r *IntentRepository) GetIntentStatistics(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count total intents
	pattern := intentStatusKeyPrefix + "*"
	keys, err := r.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get intent keys: %w", err)
	}

	stats["total_intents"] = len(keys)

	// Count expired intents
	expiredCount, err := r.redis.SCard(ctx, expiredIntentsKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get expired count: %w", err)
	}
	stats["expired_intents"] = expiredCount

	// Count by status (sample up to 1000 intents for performance)
	statusCounts := make(map[string]int)
	sampleSize := 1000
	if len(keys) > sampleSize {
		keys = keys[:sampleSize]
	}

	for _, key := range keys {
		intentID := strings.TrimPrefix(key, intentStatusKeyPrefix)
		intentStatus, err := r.GetIntentStatus(ctx, intentID)
		if err != nil {
			continue
		}
		statusCounts[intentStatus.Status]++
	}

	stats["status_counts"] = statusCounts
	stats["sampled"] = len(keys) < len(keys)

	return stats, nil
}

// CleanupExpiredIndexes removes expired intents from indexes
func (r *IntentRepository) CleanupExpiredIndexes(ctx context.Context) error {
	// Get contract index keys
	contractKeys, err := r.redis.Keys(ctx, contractIntentsKeyPrefix+"*").Result()
	if err != nil {
		return fmt.Errorf("failed to get contract keys: %w", err)
	}

	// Clean up contract indexes
	for _, contractKey := range contractKeys {
		intentIDs, err := r.redis.SMembers(ctx, contractKey).Result()
		if err != nil {
			continue
		}

		for _, intentID := range intentIDs {
			// Check if intent still exists
			statusKey := intentStatusKeyPrefix + intentID
			exists, err := r.redis.Exists(ctx, statusKey).Result()
			if err != nil || exists == 0 {
				// Remove from index
				r.redis.SRem(ctx, contractKey, intentID)
			}
		}

		// Remove empty sets
		count, err := r.redis.SCard(ctx, contractKey).Result()
		if err == nil && count == 0 {
			r.redis.Del(ctx, contractKey)
		}
	}

	// Similarly clean up tx hash indexes
	txHashKeys, err := r.redis.Keys(ctx, txHashIntentsKeyPrefix+"*").Result()
	if err != nil {
		return fmt.Errorf("failed to get tx hash keys: %w", err)
	}

	for _, txHashKey := range txHashKeys {
		intentIDs, err := r.redis.SMembers(ctx, txHashKey).Result()
		if err != nil {
			continue
		}

		for _, intentID := range intentIDs {
			statusKey := intentStatusKeyPrefix + intentID
			exists, err := r.redis.Exists(ctx, statusKey).Result()
			if err != nil || exists == 0 {
				r.redis.SRem(ctx, txHashKey, intentID)
			}
		}

		count, err := r.redis.SCard(ctx, txHashKey).Result()
		if err == nil && count == 0 {
			r.redis.Del(ctx, txHashKey)
		}
	}

	return nil
}
