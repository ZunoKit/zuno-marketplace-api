package graphql_resolver

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/utils"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/websocket"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
)

// WebSocketSubscriptionResolver implements real-time WebSocket-based subscriptions
type WebSocketSubscriptionResolver struct {
	server              *Resolver
	activeSubscriptions map[string]*intentSubscription
	mu                  sync.RWMutex
}

// intentSubscription tracks an active intent subscription
type intentSubscription struct {
	intentID   string
	statusChan chan *schemas.IntentStatusPayload
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	lastStatus *schemas.IntentStatusPayload
}

// NewWebSocketSubscriptionResolver creates a new WebSocket-based subscription resolver
func NewWebSocketSubscriptionResolver(server *Resolver) *WebSocketSubscriptionResolver {
	return &WebSocketSubscriptionResolver{
		server:              server,
		activeSubscriptions: make(map[string]*intentSubscription),
	}
}

// OnIntentStatus subscribes to real-time intent status updates via WebSocket
func (r *WebSocketSubscriptionResolver) OnIntentStatus(ctx context.Context, intentID string) (<-chan *schemas.IntentStatusPayload, error) {
	// Validate input
	if intentID == "" {
		return nil, fmt.Errorf("invalid intent ID")
	}

	if r.server.orchestratorClient == nil || r.server.orchestratorClient.Client == nil {
		return nil, fmt.Errorf("orchestrator service unavailable")
	}

	if r.server.websocketClient == nil {
		return nil, fmt.Errorf("WebSocket client not available")
	}

	// Check if WebSocket client is connected
	if !r.server.websocketClient.IsConnected() {
		return nil, fmt.Errorf("WebSocket connection not available")
	}

	log.Printf("Starting real-time subscription for intent: %s", intentID)

	// Create subscription tracking
	subCtx, cancel := context.WithCancel(ctx)
	statusChan := make(chan *schemas.IntentStatusPayload, 10) // Buffered channel

	subscription := &intentSubscription{
		intentID:   intentID,
		statusChan: statusChan,
		ctx:        subCtx,
		cancel:     cancel,
	}

	// Register subscription
	r.mu.Lock()
	r.activeSubscriptions[intentID] = subscription
	r.mu.Unlock()

	// Get and send initial status
	go func() {
		defer func() {
			if recoveryErr := recover(); recoveryErr != nil {
				log.Printf("Recovered from panic in OnIntentStatus for %s: %v", intentID, recoveryErr)
			}

			// Cleanup on exit
			r.cleanupSubscription(intentID)
			close(statusChan)
		}()

		// Fetch and send initial status
		if initialStatus, err := r.fetchCurrentStatus(subCtx, intentID); err == nil && initialStatus != nil {
			subscription.mu.Lock()
			subscription.lastStatus = initialStatus
			subscription.mu.Unlock()

			select {
			case statusChan <- initialStatus:
				log.Printf("Sent initial status for intent %s: %s", intentID, initialStatus.Status)
			case <-subCtx.Done():
				return
			}
		} else {
			log.Printf("Failed to fetch initial status for intent %s: %v", intentID, err)
		}

		// Subscribe to WebSocket updates
		wsCallback := func(wsIntentID string, data *websocket.IntentStatusData) error {
			if wsIntentID != intentID {
				return nil // Not for this subscription
			}

			// Convert WebSocket data to GraphQL schema
			payload := &schemas.IntentStatusPayload{
				IntentID: data.IntentID,
				Status:   schemas.IntentStatus(data.Status),
			}

			if data.ChainID != "" {
				payload.ChainID = &data.ChainID
			}
			if data.TxHash != "" {
				payload.TxHash = &data.TxHash
			}
			if data.ContractAddress != "" {
				payload.ContractAddress = &data.ContractAddress
			}

			// Extract kind from data if available
			if dataMap, ok := data.Data.(map[string]interface{}); ok {
				if kind, exists := dataMap["kind"].(string); exists {
					payload.Kind = kind
				}
			}

			// Check for changes to avoid duplicate notifications
			subscription.mu.RLock()
			lastStatus := subscription.lastStatus
			subscription.mu.RUnlock()

			if lastStatus != nil && r.payloadsEqual(lastStatus, payload) {
				return nil // No change, skip
			}

			// Update last status
			subscription.mu.Lock()
			subscription.lastStatus = payload
			subscription.mu.Unlock()

			// Send update
			select {
			case statusChan <- payload:
				log.Printf("Sent real-time update for intent %s: %s", intentID, payload.Status)
			case <-subCtx.Done():
				return fmt.Errorf("subscription cancelled")
			default:
				log.Printf("Channel full, dropping update for intent %s", intentID)
			}

			return nil
		}

		// Subscribe to WebSocket updates
		if err := r.server.websocketClient.Subscribe(intentID, wsCallback); err != nil {
			log.Printf("Failed to subscribe to WebSocket updates for intent %s: %v", intentID, err)
			return
		}

		log.Printf("Successfully subscribed to real-time updates for intent: %s", intentID)

		// Keep subscription alive and handle context cancellation
		<-subCtx.Done()
		log.Printf("Subscription cancelled for intent: %s", intentID)

		// Unsubscribe from WebSocket
		if err := r.server.websocketClient.Unsubscribe(intentID); err != nil {
			log.Printf("Failed to unsubscribe from WebSocket for intent %s: %v", intentID, err)
		}
	}()

	return statusChan, nil
}

// fetchCurrentStatus fetches the current status from the orchestrator service
func (r *WebSocketSubscriptionResolver) fetchCurrentStatus(ctx context.Context, intentID string) (*schemas.IntentStatusPayload, error) {
	resp, err := (*r.server.orchestratorClient.Client).GetIntentStatus(ctx, &orchestratorpb.GetIntentStatusRequest{
		IntentId: intentID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get intent status: %w", err)
	}

	payload := &schemas.IntentStatusPayload{
		IntentID: resp.IntentId,
		Kind:     resp.Kind,
		Status:   schemas.IntentStatus(resp.Status),
	}

	if resp.ChainId != "" {
		payload.ChainID = &resp.ChainId
	}
	if resp.TxHash != "" {
		payload.TxHash = &resp.TxHash
	}
	if resp.ContractAddress != "" {
		payload.ContractAddress = &resp.ContractAddress
	}

	return payload, nil
}

// payloadsEqual compares two status payloads to check for changes
func (r *WebSocketSubscriptionResolver) payloadsEqual(a, b *schemas.IntentStatusPayload) bool {
	if a.IntentID != b.IntentID || a.Status != b.Status || a.Kind != b.Kind {
		return false
	}

	if utils.PtrStr(a.ChainID) != utils.PtrStr(b.ChainID) {
		return false
	}
	if utils.PtrStr(a.TxHash) != utils.PtrStr(b.TxHash) {
		return false
	}
	if utils.PtrStr(a.ContractAddress) != utils.PtrStr(b.ContractAddress) {
		return false
	}

	return true
}

// cleanupSubscription removes a subscription from tracking
func (r *WebSocketSubscriptionResolver) cleanupSubscription(intentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if subscription, exists := r.activeSubscriptions[intentID]; exists {
		subscription.cancel()
		delete(r.activeSubscriptions, intentID)
		log.Printf("Cleaned up subscription for intent: %s", intentID)
	}
}

// GetActiveSubscriptions returns the number of active subscriptions (for monitoring)
func (r *WebSocketSubscriptionResolver) GetActiveSubscriptions() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.activeSubscriptions)
}

// Shutdown gracefully shuts down all active subscriptions
func (r *WebSocketSubscriptionResolver) Shutdown() {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("Shutting down %d active subscriptions", len(r.activeSubscriptions))

	for intentID, subscription := range r.activeSubscriptions {
		subscription.cancel()
		delete(r.activeSubscriptions, intentID)
	}

	log.Println("All subscriptions shut down")
}
