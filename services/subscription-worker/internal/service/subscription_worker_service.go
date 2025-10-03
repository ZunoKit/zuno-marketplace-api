package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/subscription-worker/internal/infrastructure/repository"
	redisClient "github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type SubscriptionWorkerService struct {
	intentRepo domain.IntentRepository
	wsManager  domain.WebSocketManager
}

// NewSubscriptionWorkerService creates a new subscription worker service
func NewSubscriptionWorkerService(
	redisClient *redisClient.Redis,
	wsManager domain.WebSocketManager,
) *SubscriptionWorkerService {
	intentRepo := repository.NewIntentRepository(redisClient)

	return &SubscriptionWorkerService{
		intentRepo: intentRepo,
		wsManager:  wsManager,
	}
}

// HandleCollectionDomainEvent processes a collection domain event from the catalog service
func (s *SubscriptionWorkerService) HandleCollectionDomainEvent(ctx context.Context, event *domain.DomainEvent) error {
	if event == nil {
		return fmt.Errorf("domain event cannot be nil")
	}

	log.Printf("Handling collection domain event: EventID=%s, Type=%s, AggregateID=%s, ChainID=%s",
		event.EventID, event.EventType, event.AggregateID, event.ChainID)

	switch event.EventType {
	case "collection_upserted", "collection_created":
		return s.ProcessCollectionUpserted(ctx, event)
	default:
		log.Printf("Unhandled event type: %s", event.EventType)
		return nil
	}
}

// ProcessCollectionUpserted processes a collection upserted/created event
func (s *SubscriptionWorkerService) ProcessCollectionUpserted(ctx context.Context, event *domain.DomainEvent) error {
	// Extract contract address from event data
	contractAddress, ok := event.Data["contract_address"].(string)
	if !ok || contractAddress == "" {
		return fmt.Errorf("contract_address not found in event data")
	}

	// Extract transaction hash if available
	txHash := ""
	if txHashRaw, exists := event.Data["tx_hash"]; exists {
		if txHashStr, ok := txHashRaw.(string); ok {
			txHash = txHashStr
		}
	}

	log.Printf("Processing collection upserted: Contract=%s, ChainID=%s, TxHash=%s",
		contractAddress, event.ChainID, txHash)

	// Primary intent resolution: Match by tx_hash first (per CREATE.md line 80)
	var resolvedIntents []*domain.IntentStatus
	
	if txHash != "" {
		txIntents, err := s.intentRepo.GetIntentsByTxHash(ctx, event.ChainID, txHash)
		if err != nil {
			log.Printf("Failed to get intents by tx hash: %v", err)
		} else {
			for _, intent := range txIntents {
				if intent.Status == "pending" || intent.Status == "processing" {
					if err := s.resolveIntentWithCollection(ctx, intent, event); err != nil {
						log.Printf("Failed to resolve intent %s by tx hash: %v", intent.IntentID, err)
						continue
					}
					resolvedIntents = append(resolvedIntents, intent)
				}
			}
		}
	}

	// Fallback: Find pending intents for this contract if no tx_hash match
	if len(resolvedIntents) == 0 {
		pendingIntents, err := s.intentRepo.GetPendingIntentsByContract(ctx, event.ChainID, contractAddress)
		if err != nil {
			return fmt.Errorf("failed to get pending intents: %w", err)
		}

		log.Printf("Found %d pending intents for contract %s", len(pendingIntents), contractAddress)

		// Resolve matching intents
		for _, intent := range pendingIntents {
			if err := s.resolveIntentWithCollection(ctx, intent, event); err != nil {
				log.Printf("Failed to resolve intent %s: %v", intent.IntentID, err)
				continue
			}
			resolvedIntents = append(resolvedIntents, intent)
		}
	}

	if len(resolvedIntents) == 0 {
		log.Printf("No matching intents found for collection event: contract=%s, chain=%s, tx=%s", 
			contractAddress, event.ChainID, txHash)
	} else {
		log.Printf("Successfully resolved %d intents", len(resolvedIntents))
	}

	return nil
}

// resolveIntentWithCollection resolves an intent using collection data
func (s *SubscriptionWorkerService) resolveIntentWithCollection(ctx context.Context, intent *domain.IntentStatus, event *domain.DomainEvent) error {
	// Update intent status to ready (per CREATE.md line 82)
	intent.Status = "ready"
	intent.ContractAddress = event.Data["contract_address"].(string)
	intent.ChainID = event.ChainID
	
	// Add collection data to intent
	if intent.Data == nil {
		intent.Data = make(map[string]interface{})
	}
	
	// Copy relevant collection data
	intent.Data["collection_id"] = event.AggregateID
	intent.Data["collection_name"] = event.Data["name"]
	intent.Data["collection_symbol"] = event.Data["symbol"]
	intent.Data["contract_address"] = event.Data["contract_address"]
	intent.Data["creator"] = event.Data["creator"]
	intent.Data["collection_type"] = event.Data["collection_type"]
	intent.Data["chain_id"] = event.ChainID
	
	if txHash, exists := event.Data["tx_hash"]; exists {
		intent.TxHash = txHash.(string)
		intent.Data["tx_hash"] = txHash
	}

	intent.UpdatedAt = time.Now()

	// Update intent in Redis
	if err := s.intentRepo.UpdateIntentStatus(ctx, intent); err != nil {
		return fmt.Errorf("failed to update intent status: %w", err)
	}

	// Notify WebSocket subscribers
	if err := s.ResolveIntent(ctx, intent.IntentID, intent); err != nil {
		log.Printf("Failed to notify WebSocket subscribers for intent %s: %v", intent.IntentID, err)
	}

	log.Printf("Successfully resolved intent: %s -> %s", intent.IntentID, intent.Status)
	return nil
}

// ResolveIntent resolves an intent and notifies subscribers
func (s *SubscriptionWorkerService) ResolveIntent(ctx context.Context, intentID string, status *domain.IntentStatus) error {
	// Create WebSocket message
	message := domain.NewStatusUpdateMessage(intentID, status)

	// Send to all subscribers of this intent
	if err := s.wsManager.SendToIntent(intentID, message); err != nil {
		return fmt.Errorf("failed to send WebSocket message: %w", err)
	}

	log.Printf("Sent intent resolution notification to WebSocket subscribers: %s", intentID)
	return nil
}

// SubscribeToIntent subscribes a WebSocket connection to an intent
func (s *SubscriptionWorkerService) SubscribeToIntent(ctx context.Context, connID, intentID string) error {
	// Verify intent exists
	_, err := s.intentRepo.GetIntentStatus(ctx, intentID)
	if err != nil {
		return fmt.Errorf("intent not found: %w", err)
	}

	// Note: Actual subscription management is handled by WebSocket manager
	// This is just for validation and logging
	log.Printf("Connection %s subscribed to intent %s", connID, intentID)
	return nil
}

// UnsubscribeFromIntent unsubscribes a WebSocket connection from an intent
func (s *SubscriptionWorkerService) UnsubscribeFromIntent(ctx context.Context, connID, intentID string) error {
	// Note: Actual subscription management is handled by WebSocket manager
	// This is just for logging
	log.Printf("Connection %s unsubscribed from intent %s", connID, intentID)
	return nil
}

// CleanupExpiredIntents cleans up expired intents
func (s *SubscriptionWorkerService) CleanupExpiredIntents(ctx context.Context) (int, error) {
	expiredIntents, err := s.intentRepo.GetExpiredIntents(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get expired intents: %w", err)
	}

	cleanedCount := 0
	for _, intent := range expiredIntents {
		// Notify subscribers that the intent has expired
		expiredMessage := domain.NewErrorMessage(intent.IntentID, "Intent has expired")
		if err := s.wsManager.SendToIntent(intent.IntentID, expiredMessage); err != nil {
			log.Printf("Failed to notify expiry for intent %s: %v", intent.IntentID, err)
		}

		// Delete the intent
		if err := s.intentRepo.DeleteIntent(ctx, intent.IntentID); err != nil {
			log.Printf("Failed to delete expired intent %s: %v", intent.IntentID, err)
			continue
		}

		cleanedCount++
	}

	if cleanedCount > 0 {
		log.Printf("Cleaned up %d expired intents", cleanedCount)
	}

	return cleanedCount, nil
}

// HealthCheck performs a health check on the subscription worker service
func (s *SubscriptionWorkerService) HealthCheck(ctx context.Context) error {
	// Check intent repository
	if err := s.intentRepo.HealthCheck(ctx); err != nil {
		return fmt.Errorf("intent repository unhealthy: %w", err)
	}

	// Check WebSocket manager
	if err := s.wsManager.HealthCheck(); err != nil {
		return fmt.Errorf("WebSocket manager unhealthy: %w", err)
	}

	return nil
}

// GetServiceStats returns statistics about the service
func (s *SubscriptionWorkerService) GetServiceStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get WebSocket connection count
	stats["websocket_connections"] = s.wsManager.GetConnectionCount()

	// Get intent statistics (implement in intent repository)
	if statsRepo, ok := s.intentRepo.(*repository.IntentRepository); ok {
		intentStats, err := statsRepo.GetIntentStatistics(ctx)
		if err != nil {
			log.Printf("Failed to get intent statistics: %v", err)
		} else {
			stats["intents"] = intentStats
		}
	}

	// Add service health
	if err := s.HealthCheck(ctx); err != nil {
		stats["health"] = "unhealthy"
		stats["health_error"] = err.Error()
	} else {
		stats["health"] = "healthy"
	}

	return stats, nil
}

// ProcessIntentUpdate handles manual intent updates (for testing/admin purposes)
func (s *SubscriptionWorkerService) ProcessIntentUpdate(ctx context.Context, intentID string, updates map[string]interface{}) error {
	// Get current intent
	intent, err := s.intentRepo.GetIntentStatus(ctx, intentID)
	if err != nil {
		return fmt.Errorf("failed to get intent: %w", err)
	}

	// Apply updates
	updated := false

	if status, ok := updates["status"].(string); ok {
		intent.Status = status
		updated = true
	}

	if txHash, ok := updates["tx_hash"].(string); ok {
		intent.TxHash = txHash
		updated = true
	}

	if contractAddress, ok := updates["contract_address"].(string); ok {
		intent.ContractAddress = contractAddress
		updated = true
	}

	if data, ok := updates["data"].(map[string]interface{}); ok {
		if intent.Data == nil {
			intent.Data = make(map[string]interface{})
		}
		for k, v := range data {
			intent.Data[k] = v
		}
		updated = true
	}

	if !updated {
		return fmt.Errorf("no valid updates provided")
	}

	intent.UpdatedAt = time.Now()

	// Save updates
	if err := s.intentRepo.UpdateIntentStatus(ctx, intent); err != nil {
		return fmt.Errorf("failed to update intent: %w", err)
	}

	// Notify subscribers
	if err := s.ResolveIntent(ctx, intentID, intent); err != nil {
		log.Printf("Failed to notify subscribers of intent update: %v", err)
	}

	log.Printf("Processed intent update: %s", intentID)
	return nil
}

// MonitorIntents starts a monitoring routine for intents
func (s *SubscriptionWorkerService) MonitorIntents(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	log.Println("Starting intent monitoring routine")

	for {
		select {
		case <-ctx.Done():
			log.Println("Intent monitoring stopped")
			return ctx.Err()

		case <-ticker.C:
			// Cleanup expired intents
			cleaned, err := s.CleanupExpiredIntents(ctx)
			if err != nil {
				log.Printf("Error during intent cleanup: %v", err)
			} else if cleaned > 0 {
				log.Printf("Intent cleanup completed: %d intents cleaned", cleaned)
			}

			// Cleanup expired indexes in repository
			if statsRepo, ok := s.intentRepo.(*repository.IntentRepository); ok {
				if err := statsRepo.CleanupExpiredIndexes(ctx); err != nil {
					log.Printf("Error during index cleanup: %v", err)
				}
			}
		}
	}
}

// Helper function to match intent criteria with collection data
func (s *SubscriptionWorkerService) matchesIntentCriteria(intent *domain.IntentStatus, collectionData map[string]interface{}) bool {
	// Basic matching logic - in a real implementation, this would be more sophisticated
	
	// Match by contract address
	if intent.ContractAddress != "" {
		if contractAddr, ok := collectionData["contract_address"].(string); ok {
			if !strings.EqualFold(intent.ContractAddress, contractAddr) {
				return false
			}
		}
	}

	// Match by chain ID
	if intent.ChainID != "" {
		if chainID, ok := collectionData["chain_id"].(string); ok {
			if intent.ChainID != chainID {
				return false
			}
		}
	}

	// Match by transaction hash if available
	if intent.TxHash != "" {
		if txHash, ok := collectionData["tx_hash"].(string); ok {
			if !strings.EqualFold(intent.TxHash, txHash) {
				return false
			}
		}
	}

	// Additional criteria can be added based on intent.Data content
	
	return true
}