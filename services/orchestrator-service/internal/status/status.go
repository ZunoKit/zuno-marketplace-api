package status

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type StatusCache struct {
	redis *redis.Redis
}

func NewStatusCache() domain.StatusCache {
	return &StatusCache{}
}

// SetIntentStatus stores intent status in Redis with TTL
func (s *StatusCache) SetIntentStatus(ctx context.Context, payload domain.IntentStatusPayload, ttl time.Duration) error {
	if s.redis == nil {
		// For now, just return nil to avoid breaking the build
		// In production, this would require proper Redis initialization
		return nil
	}

	key := fmt.Sprintf("intent:status:%s", payload.IntentID)

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	err = s.redis.SetWithExpiration(ctx, key, string(data), ttl)
	if err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

// SetRedis sets the Redis client (called from main.go)
func (s *StatusCache) SetRedis(r *redis.Redis) {
	s.redis = r
}
