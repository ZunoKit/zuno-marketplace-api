package events

import (
	"context"
	"encoding/json"
	"log"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/contracts"
	"github.com/quangdang46/NFT-Marketplace/shared/messaging"
)

type EventPublisher struct {
	rabbitmq *messaging.RabbitMQ
}

func NewEventPublisher(rabbitmq *messaging.RabbitMQ) domain.WalletEventPublisher {
	return &EventPublisher{
		rabbitmq: rabbitmq,
	}
}

func (p *EventPublisher) PublishWalletLinked(ctx context.Context, event *domain.WalletLinkedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	message := contracts.AMQPMessage{
		Exchange:   contracts.WalletsExchange,
		RoutingKey: contracts.WalletLinkedKey,
		Body:       body,
	}

	if err := p.rabbitmq.Publish(ctx, message); err != nil {
		log.Printf("Failed to publish wallet.linked event: %v", err)
		return err
	}

	log.Printf("Published wallet.linked event for user %s, wallet %s", event.UserID, event.WalletID)
	return nil
}
