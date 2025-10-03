package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/contracts"
)

// EventPublisher publishes auth service domain events to AMQP
type EventPublisher struct {
	amqp contracts.AMQPClient
}

func NewEventPublisher(amqp contracts.AMQPClient) *EventPublisher {
	return &EventPublisher{amqp: amqp}
}

// PublishUserLoggedIn publishes an auth.user_logged_in event after successful login
func (p *EventPublisher) PublishUserLoggedIn(ctx context.Context, event *domain.AuthUserLoggedInEvent) error {
	if p.amqp == nil {
		// AMQP is optional in development; skip publishing when not configured
		fmt.Printf("AMQP not available, skipping user_logged_in event: %+v\n", event)
		return nil
	}

	payload := map[string]interface{}{
		"user_id":     event.UserID,
		"account_id":  event.AccountID,
		"address":     event.Address,
		"chain_id":    event.ChainID,
		"session_id":  event.SessionID,
		"logged_in_at": event.LoggedInAt.Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal user_logged_in event: %w", err)
	}

	return p.amqp.Publish(ctx, contracts.AMQPMessage{
		Exchange:   contracts.AuthExchange,
		RoutingKey: contracts.UserLoggedInKey,
		Body:       body,
		Headers: map[string]interface{}{
			"event_type":   "user.logged_in",
			"schema":       "auth.user_logged_in.v1",
			"published_at": time.Now().Format(time.RFC3339),
			"service":      "auth-service",
		},
	})
}
