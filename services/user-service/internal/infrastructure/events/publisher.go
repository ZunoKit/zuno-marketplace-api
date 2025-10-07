package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/contracts"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
)

type EventPublisher struct {
	rabbitmq *messaging.RabbitMQ
}

func NewEventPublisher(rabbitmq *messaging.RabbitMQ) domain.UserEventPublisher {
	return &EventPublisher{
		rabbitmq: rabbitmq,
	}
}

func (p *EventPublisher) PublishUserCreated(ctx context.Context, event *domain.UserCreatedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	message := contracts.AMQPMessage{
		Exchange:   contracts.UsersExchange,
		RoutingKey: "user.created",
		Body:       body,
	}

	if err := p.rabbitmq.Publish(ctx, message); err != nil {
		log.Printf("Failed to publish user.created event: %v", err)
		return err
	}

	log.Printf("Published user.created event for user %s", event.UserID)
	return nil
}
