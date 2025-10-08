package graphql_resolver

import (
	"context"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
)

// SubscriptionResolver handles GraphQL subscriptions
type SubscriptionResolver struct {
	server *Resolver
}

// OnMintStatus subscribes to mint status updates for a specific intent
func (r *SubscriptionResolver) OnMintStatus(ctx context.Context, intentID string) (<-chan *schemas.MintStatus, error) {
	// Validate intent ID
	if intentID == "" {
		return nil, fmt.Errorf("intent ID is required")
	}

	// Get current user for authorization (optional)
	user := middleware.GetCurrentUser(ctx)
	subscriberID := "anonymous"
	if user != nil {
		subscriberID = user.UserID
	}

	// Create channel for status updates
	statusChan := make(chan *schemas.MintStatus, 1)

	// Check WebSocket client availability
	if r.server.websocketClient == nil {
		close(statusChan)
		return nil, fmt.Errorf("WebSocket service unavailable")
	}

	// Subscribe to intent updates
	subscription, err := r.server.websocketClient.Subscribe(ctx, intentID, subscriberID)
	if err != nil {
		close(statusChan)
		return nil, fmt.Errorf("failed to subscribe to intent %s: %w", intentID, err)
	}

	// Start goroutine to handle updates
	go func() {
		defer close(statusChan)
		defer subscription.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-subscription.Messages:
				if !ok {
					return // Subscription closed
				}

				// Convert message to MintStatus
				status, err := r.parseMintStatus(msg)
				if err != nil {
					fmt.Printf("Error parsing mint status: %v\n", err)
					continue
				}

				// Send status update
				select {
				case statusChan <- status:
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					// Timeout sending to channel
					fmt.Printf("Timeout sending status update for intent %s\n", intentID)
				}
			}
		}
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
		return nil, fmt.Errorf("intent ID is required")
	}

	// Get current user for authorization
	user := middleware.GetCurrentUser(ctx)
	subscriberID := "anonymous"
	if user != nil {
		subscriberID = user.UserID
	}

	// Create channel for status updates
	statusChan := make(chan *schemas.CollectionStatus, 1)

	// Check WebSocket client availability
	if r.server.websocketClient == nil {
		close(statusChan)
		return nil, fmt.Errorf("WebSocket service unavailable")
	}

	// Subscribe to intent updates
	subscription, err := r.server.websocketClient.Subscribe(ctx, intentID, subscriberID)
	if err != nil {
		close(statusChan)
		return nil, fmt.Errorf("failed to subscribe to intent %s: %w", intentID, err)
	}

	// Start goroutine to handle updates
	go func() {
		defer close(statusChan)
		defer subscription.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-subscription.Messages:
				if !ok {
					return
				}

				// Convert message to CollectionStatus
				status, err := r.parseCollectionStatus(msg)
				if err != nil {
					fmt.Printf("Error parsing collection status: %v\n", err)
					continue
				}

				// Send status update
				select {
				case statusChan <- status:
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					fmt.Printf("Timeout sending status update for intent %s\n", intentID)
				}
			}
		}
	}()

	return statusChan, nil
}

// OnIntentStatus subscribes to generic intent status updates
func (r *SubscriptionResolver) OnIntentStatus(ctx context.Context, intentID string) (<-chan *schemas.IntentStatusPayload, error) {
	// Validate intent ID
	if intentID == "" {
		return nil, fmt.Errorf("intent ID is required")
	}

	// Get current user
	user := middleware.GetCurrentUser(ctx)
	subscriberID := "anonymous"
	if user != nil {
		subscriberID = user.UserID
	}

	// Create channel for status updates
	statusChan := make(chan *schemas.IntentStatusPayload, 1)

	// Check WebSocket client availability
	if r.server.websocketClient == nil {
		close(statusChan)
		return nil, fmt.Errorf("WebSocket service unavailable")
	}

	// Subscribe to intent updates
	subscription, err := r.server.websocketClient.Subscribe(ctx, intentID, subscriberID)
	if err != nil {
		close(statusChan)
		return nil, fmt.Errorf("failed to subscribe to intent %s: %w", intentID, err)
	}

	// Start goroutine to handle updates
	go func() {
		defer close(statusChan)
		defer subscription.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-subscription.Messages:
				if !ok {
					return
				}

				// Convert message to IntentStatusPayload
				status, err := r.parseIntentStatus(msg)
				if err != nil {
					fmt.Printf("Error parsing intent status: %v\n", err)
					continue
				}

				// Send status update
				select {
				case statusChan <- status:
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					fmt.Printf("Timeout sending status update for intent %s\n", intentID)
				}
			}
		}
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
		ChainID:  schemas.ChainID("eip155:1"),
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
		ChainID:  schemas.ChainID("eip155:1"),
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
