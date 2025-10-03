package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/contracts"
)

type EventPublisher struct {
	amqp contracts.AMQPClient
}

func NewEventPublisher(amqp contracts.AMQPClient) *EventPublisher {
	return &EventPublisher{
		amqp: amqp,
	}
}

// PublishWalletLinked publishes a wallet linked event
func (p *EventPublisher) PublishWalletLinked(ctx context.Context, event *domain.WalletLinkedEvent) error {
	// Skip publishing if AMQP is not available
	if p.amqp == nil {
		fmt.Printf("AMQP not available, skipping wallet linked event: %+v\n", event)
		return nil
	}

	// Create the message payload
	payload := domain.WalletLinkedEvent{
		UserID:    event.UserID,
		AccountID: event.AccountID,
		WalletID:  event.WalletID,
		Address:   event.Address,
		ChainID:   event.ChainID,
		IsPrimary: event.IsPrimary,
		LinkedAt:  event.LinkedAt,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal wallet linked event: %w", err)
	}

	// Publish to wallets.events exchange with routing key "wallet.linked"
	if err := p.amqp.Publish(ctx, contracts.AMQPMessage{
		Exchange:   contracts.WalletsExchange,
		RoutingKey: contracts.WalletLinkedKey,
		Body:       body,
		Headers: map[string]interface{}{
			"event_type":   "wallet.linked",
			"schema":       "wallet.linked.v1",
			"published_at": time.Now().Format(time.RFC3339),
			"service":      "wallet-service",
		},
	}); err != nil {
		return fmt.Errorf("failed to publish wallet linked event: %w", err)
	}
	return nil
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return fmt.Sprintf("wallet_%d", time.Now().UnixNano())
}

// GenerateEventID exposes event ID generation for testing
func GenerateEventID() string { return generateEventID() }
