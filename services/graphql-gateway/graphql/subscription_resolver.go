package graphql_resolver

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/websocket"
)

// SubscriptionResolver handles GraphQL subscriptions
type SubscriptionResolver struct {
	server *Resolver
}

// OnMintStatus subscribes to mint status updates for a specific intent
func (r *SubscriptionResolver) OnMintStatus(ctx context.Context, intentID string) (<-chan *schemas.MintStatus, error) {
	// Validate intent ID
	if intentID == "" {
		return nil, fmt.Errorf("invalid intent ID: cannot be empty")
	}

	// Get current user for authorization (optional)
	_ = middleware.GetCurrentUser(ctx)

	// Create channel for status updates
	statusChan := make(chan *schemas.MintStatus, 1)

	// Check WebSocket client availability
	if r.server.websocketClient == nil {
		close(statusChan)
		return nil, fmt.Errorf("WebSocket service unavailable")
	}

	// Subscribe to intent updates
	err := r.server.websocketClient.Subscribe(intentID, func(id string, data *websocket.IntentStatusData) error {
		// Convert IntentStatusData to MintStatus
		status := &schemas.MintStatus{
			IntentID: id,
			Status:   mapWebSocketStatusToSchema(data.Status),
		}
		select {
		case statusChan <- status:
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		}
		return nil
	})
	if err != nil {
		close(statusChan)
		return nil, fmt.Errorf("failed to subscribe to intent %s: %w", intentID, err)
	}

	// Start goroutine to handle context cancellation
	go func() {
		<-ctx.Done()
		close(statusChan)
		// TODO: Implement unsubscribe when context is cancelled
	}()

	// Send initial status immediately if available
	go func() {
		initialStatus, err := r.getInitialMintStatus(ctx, intentID)
		if err == nil && initialStatus != nil {
			select {
			case statusChan <- initialStatus:
			case <-time.After(1 * time.Second):
			}
		}
	}()

	return statusChan, nil
}

// OnCollectionStatus subscribes to collection creation status updates
func (r *SubscriptionResolver) OnCollectionStatus(ctx context.Context, intentID string) (<-chan *schemas.CollectionStatus, error) {
	// Validate intent ID
	if intentID == "" {
		return nil, fmt.Errorf("invalid intent ID: cannot be empty")
	}

	// Get current user for authorization
	_ = middleware.GetCurrentUser(ctx)

	// Create channel for status updates
	statusChan := make(chan *schemas.CollectionStatus, 1)

	// Check WebSocket client availability
	if r.server.websocketClient == nil {
		close(statusChan)
		return nil, fmt.Errorf("WebSocket service unavailable")
	}

	// Subscribe to intent updates
	err := r.server.websocketClient.Subscribe(intentID, func(id string, data *websocket.IntentStatusData) error {
		// Convert IntentStatusData to CollectionStatus
		status := &schemas.CollectionStatus{
			IntentID: id,
			Status:   data.Status, // Use string directly for CollectionStatus
		}
		select {
		case statusChan <- status:
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		}
		return nil
	})
	if err != nil {
		close(statusChan)
		return nil, fmt.Errorf("failed to subscribe to intent %s: %w", intentID, err)
	}

	// Start goroutine to handle context cancellation
	go func() {
		<-ctx.Done()
		close(statusChan)
		// TODO: Implement unsubscribe when context is cancelled
	}()

	return statusChan, nil
}

// OnIntentStatus subscribes to generic intent status updates
func (r *SubscriptionResolver) OnIntentStatus(ctx context.Context, intentID string) (<-chan *schemas.IntentStatusPayload, error) {
	// Validate intent ID
	if intentID == "" {
		return nil, fmt.Errorf("invalid intent ID: cannot be empty")
	}

	// Get current user
	_ = middleware.GetCurrentUser(ctx)

	// Create channel for status updates
	statusChan := make(chan *schemas.IntentStatusPayload, 1)

	// Check WebSocket client availability
	if r.server.websocketClient == nil {
		close(statusChan)
		return nil, fmt.Errorf("WebSocket service unavailable")
	}

	// Subscribe to intent updates
	err := r.server.websocketClient.Subscribe(intentID, func(id string, data *websocket.IntentStatusData) error {
		// Convert IntentStatusData to IntentStatusPayload
		status := &schemas.IntentStatusPayload{
			IntentID: id,
			Status:   mapWebSocketStatusToSchema(data.Status),
			Kind:     "generic", // Default kind
		}
		select {
		case statusChan <- status:
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")
		}
		return nil
	})
	if err != nil {
		close(statusChan)
		return nil, fmt.Errorf("failed to subscribe to intent %s: %w", intentID, err)
	}

	// Start goroutine to handle context cancellation
	go func() {
		<-ctx.Done()
		close(statusChan)
		// TODO: Implement unsubscribe when context is cancelled
	}()

	// Send initial status
	go func() {
		initialStatus, err := r.getInitialIntentStatus(ctx, intentID)
		if err == nil && initialStatus != nil {
			select {
			case statusChan <- initialStatus:
			case <-time.After(1 * time.Second):
			}
		}
	}()

	return statusChan, nil
}

// Helper methods

func (r *SubscriptionResolver) parseMintStatus(msg interface{}) (*schemas.MintStatus, error) {
	// Parse the WebSocket message into MintStatus
	// This would depend on the actual message format from the WebSocket service

	// Placeholder implementation
	return &schemas.MintStatus{
		Status:   schemas.IntentStatusPending,
		IntentID: "placeholder",
		ChainID:  strPtr("eip155:1"),
	}, nil
}

func (r *SubscriptionResolver) parseCollectionStatus(msg interface{}) (*schemas.CollectionStatus, error) {
	// Parse the WebSocket message into CollectionStatus

	// Placeholder implementation
	return &schemas.CollectionStatus{
		Status:   "pending",
		IntentID: "placeholder",
	}, nil
}

func (r *SubscriptionResolver) parseIntentStatus(msg interface{}) (*schemas.IntentStatusPayload, error) {
	// Parse the WebSocket message into IntentStatusPayload

	// Placeholder implementation
	return &schemas.IntentStatusPayload{
		IntentID: "placeholder",
		Kind:     "mint",
		Status:   schemas.IntentStatusPending,
	}, nil
}

func (r *SubscriptionResolver) getInitialMintStatus(ctx context.Context, intentID string) (*schemas.MintStatus, error) {
	// Get initial status from orchestrator service
	if r.server.orchestratorClient == nil {
		return nil, fmt.Errorf("orchestrator service unavailable")
	}

	// Call GetIntentStatus on orchestrator
	// This would need to be implemented in the orchestrator client

	// Placeholder implementation
	return &schemas.MintStatus{
		Status:   schemas.IntentStatusPending,
		IntentID: intentID,
		ChainID:  strPtr("eip155:1"),
	}, nil
}

func (r *SubscriptionResolver) getInitialIntentStatus(ctx context.Context, intentID string) (*schemas.IntentStatusPayload, error) {
	// Get initial status from orchestrator service
	if r.server.orchestratorClient == nil {
		return nil, fmt.Errorf("orchestrator service unavailable")
	}

	// Placeholder implementation
	return &schemas.IntentStatusPayload{
		IntentID: intentID,
		Kind:     "mint",
		Status:   schemas.IntentStatusPending,
	}, nil
}

// OnMediaPinned subscribes to media pinning status updates
func (r *SubscriptionResolver) OnMediaPinned(ctx context.Context, assetID string) (<-chan *schemas.MediaAsset, error) {
	ch := make(chan *schemas.MediaAsset, 1)

	// TODO: Implement actual subscription to media service events
	// For now, return a placeholder channel
	go func() {
		defer close(ch)
		// Placeholder: send initial status
		ch <- &schemas.MediaAsset{
			ID:        assetID,
			PinStatus: schemas.PinStatusPinning,
			Kind:      schemas.MediaKindImage, // Default
			Mime:      "image/jpeg",
			Sha256:    "placeholder",
			CreatedAt: time.Now().Format(time.RFC3339),
			RefCount:  1,
			Variants:  []*schemas.MediaVariant{},
		}
	}()

	return ch, nil
}

// Helper function to generate subscriber ID
func generateSubscriberID() string {
	return uuid.New().String()
}

// Helper function to map WebSocket status to schema status
func mapWebSocketStatusToSchema(status string) schemas.IntentStatus {
	switch status {
	case "pending":
		return schemas.IntentStatusPending
	case "ready":
		return schemas.IntentStatusReady
	case "failed":
		return schemas.IntentStatusFailed
	case "expired":
		return schemas.IntentStatusExpired
	default:
		return schemas.IntentStatusPending
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
