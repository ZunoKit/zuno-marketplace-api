package graphql_resolver

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/utils"
	orchestratorpb "github.com/quangdang46/NFT-Marketplace/shared/proto/orchestrator"
)

type SubscriptionResolver struct {
	server *Resolver
}

func (r *SubscriptionResolver) OnIntentStatus(ctx context.Context, intentID string) (<-chan *schemas.IntentStatusPayload, error) {
	// Validate input
	if intentID == "" {
		return nil, fmt.Errorf("invalid intent ID")
	}

	if r.server.orchestratorClient == nil || r.server.orchestratorClient.Client == nil {
		return nil, fmt.Errorf("orchestrator service unavailable")
	}

	// Try real-time WebSocket subscription first
	if r.server.websocketClient != nil && r.server.websocketClient.IsConnected() {
		log.Printf("Using real-time WebSocket subscription for intent: %s", intentID)
		enhancedResolver := NewEnhancedSubscriptionResolver(r.server)
		return enhancedResolver.OnIntentStatus(ctx, intentID)
	}

	// Fallback to polling if WebSocket is not available
	log.Printf("WebSocket not available, falling back to polling for intent: %s", intentID)
	return r.onIntentStatusPolling(ctx, intentID)
}

// onIntentStatusPolling implements the original polling-based subscription
func (r *SubscriptionResolver) onIntentStatusPolling(ctx context.Context, intentID string) (<-chan *schemas.IntentStatusPayload, error) {

	// Create channel for streaming updates
	statusChan := make(chan *schemas.IntentStatusPayload)

	// Start goroutine to handle the subscription with light polling
	go func() {
		defer close(statusChan)

		// Helper to fetch and push current status
		pushStatus := func() (*schemas.IntentStatusPayload, error) {
			resp, err := (*r.server.orchestratorClient.Client).GetIntentStatus(ctx, &orchestratorpb.GetIntentStatusRequest{IntentId: intentID})
			if err != nil {
				// Log error but keep subscription alive
				fmt.Printf("GetIntentStatus error: %v\n", err)
				return nil, err
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

		// Send initial snapshot
		last := ""
		if p, err := pushStatus(); err == nil && p != nil {
			// Compose a simple fingerprint to reduce duplicates
			fp := fmt.Sprintf("%s|%s|%s", p.Status, utils.PtrStr(p.TxHash), utils.PtrStr(p.ContractAddress))
			last = fp
			select {
			case statusChan <- p:
			case <-ctx.Done():
				return
			}
		}

		// Poll periodically for changes
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p, err := pushStatus()
				if err != nil || p == nil {
					continue
				}
				fp := fmt.Sprintf("%s|%s|%s", p.Status, utils.PtrStr(p.TxHash), utils.PtrStr(p.ContractAddress))
				if fp == last {
					continue // no change
				}
				last = fp
				select {
				case statusChan <- p:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return statusChan, nil
}
