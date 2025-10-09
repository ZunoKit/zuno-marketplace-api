package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/infrastructure/repository"
)

type CatalogService struct {
	collectionRepo     domain.CollectionsRepository
	processedEventRepo domain.ProcessedEventsRepository
	publisher          domain.MessagePublisher
	unitOfWork         domain.UnitOfWork
}

// NewCatalogService creates a new catalog service
func NewCatalogService(
	collectionRepo domain.CollectionsRepository,
	processedEventRepo domain.ProcessedEventsRepository,
	publisher domain.MessagePublisher,
) *CatalogService {
	unitOfWork := repository.NewUnitOfWork(collectionRepo, processedEventRepo)

	return &CatalogService{
		collectionRepo:     collectionRepo,
		processedEventRepo: processedEventRepo,
		publisher:          publisher,
		unitOfWork:         unitOfWork,
	}
}

// HandleCollectionCreated handles collection creation events
func (s *CatalogService) HandleCollectionCreated(ctx context.Context, evt *domain.CollectionEvent) error {
	// Extract collection data from event first (before transaction)
	collection, err := s.extractCollectionFromEvent(evt)
	if err != nil {
		return fmt.Errorf("failed to extract collection from event: %w", err)
	}

	// Use unit of work to ensure data consistency - ALL operations in one transaction
	err = s.unitOfWork.WithinTx(ctx, func(ctx context.Context, tx domain.Tx) error {
		// Check if event has already been processed (INSIDE transaction)
		processed, err := tx.ProcessedEventsRepo().MarkProcessed(ctx, evt.EventID)
		if err != nil {
			return fmt.Errorf("failed to check if event is processed: %w", err)
		}

		if !processed {
			// Event already processed, skip (but don't error - idempotent)
			return nil
		}

		// Upsert collection
		created, err := tx.CollectionsRepo().Upsert(ctx, collection)
		if err != nil {
			return fmt.Errorf("failed to upsert collection: %w", err)
		}

		// Publish domain event - always publish "upserted" per CREATE.md line 74
		// Note: This should ideally be done after transaction commits
		// to avoid publishing events for failed transactions
		err = s.publishCollectionUpsertedEvent(ctx, &collection, created)
		if err != nil {
			// Log error but don't fail transaction - events are eventually consistent
			// In production, use outbox pattern for transactional messaging
			log.Printf("Warning: failed to publish collection upserted event: %v", err)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to process collection event: %w", err)
	}

	return nil
}

// extractCollectionFromEvent extracts collection data from an event
func (s *CatalogService) extractCollectionFromEvent(evt *domain.CollectionEvent) (domain.Collection, error) {
	collection := domain.Collection{
		ChainID:         evt.ChainID,
		ContractAddress: evt.Contract,
		TxHash:          evt.TxHash,
		CreatedAt:       evt.Timestamp,
		UpdatedAt:       evt.Timestamp,
	}

	// Extract required fields from event data
	if collectionAddress, ok := evt.Data["collection_address"].(string); ok {
		collection.ContractAddress = collectionAddress
	}

	if creator, ok := evt.Data["creator"].(string); ok {
		collection.Creator = creator
	}

	if name, ok := evt.Data["name"].(string); ok {
		collection.Name = name
		collection.Slug = s.generateSlug(name)
	}

	if collectionType, ok := evt.Data["collection_type"].(string); ok {
		collection.CollectionType = collectionType
	}

	// Extract optional fields
	if description, ok := evt.Data["description"].(string); ok {
		collection.Description = description
	}

	if maxSupplyStr, ok := evt.Data["max_supply"].(string); ok {
		maxSupply := new(big.Int)
		maxSupply.SetString(maxSupplyStr, 10)
		collection.MaxSupply = maxSupply
	}

	if totalSupplyStr, ok := evt.Data["total_supply"].(string); ok {
		totalSupply := new(big.Int)
		totalSupply.SetString(totalSupplyStr, 10)
		collection.TotalSupply = totalSupply
	}

	if royaltyRecipient, ok := evt.Data["royalty_recipient"].(string); ok {
		collection.RoyaltyRecipient = royaltyRecipient
	}

	if royaltyPercentageStr, ok := evt.Data["royalty_percentage"].(string); ok {
		if royaltyPercentage, err := strconv.ParseUint(royaltyPercentageStr, 10, 16); err == nil {
			collection.RoyaltyPercentage = uint16(royaltyPercentage)
		}
	}

	// Extract URLs
	if imageURL, ok := evt.Data["image_url"].(string); ok {
		collection.ImageURL = imageURL
	}

	if bannerURL, ok := evt.Data["banner_url"].(string); ok {
		collection.BannerURL = bannerURL
	}

	if externalURL, ok := evt.Data["external_url"].(string); ok {
		collection.ExternalURL = externalURL
	}

	// Extract social links
	if discordURL, ok := evt.Data["discord_url"].(string); ok {
		collection.DiscordURL = discordURL
	}

	if twitterURL, ok := evt.Data["twitter_url"].(string); ok {
		collection.TwitterURL = twitterURL
	}

	if instagramURL, ok := evt.Data["instagram_url"].(string); ok {
		collection.InstagramURL = instagramURL
	}

	if telegramURL, ok := evt.Data["telegram_url"].(string); ok {
		collection.TelegramURL = telegramURL
	}

	// Set default values
	if collection.ID == "" {
		collection.ID = uuid.New().String()
	}

	if collection.Slug == "" {
		collection.Slug = s.generateSlug(collection.Name)
	}

	return collection, nil
}

// generateSlug generates a URL-friendly slug from a name
func (s *CatalogService) generateSlug(name string) string {
	// Convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove special characters except hyphens
	var result strings.Builder
	for _, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '-' {
			result.WriteRune(char)
		}
	}

	// Remove multiple consecutive hyphens
	slug = result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")

	return slug
}

// publishCollectionCreatedEvent publishes a collection created domain event
func (s *CatalogService) publishCollectionCreatedEvent(ctx context.Context, collection *domain.Collection) error {
	domainEvent := &domain.DomainEvent{
		Schema:      "marketplace.domain.v1",
		Version:     "1.0",
		EventID:     fmt.Sprintf("collection_created_%s_%d", collection.ID, time.Now().UnixNano()),
		EventType:   "collection_created",
		AggregateID: collection.ID,
		ChainID:     collection.ChainID,
		Data: map[string]interface{}{
			"id":                 collection.ID,
			"slug":               collection.Slug,
			"name":               collection.Name,
			"chain_id":           collection.ChainID,
			"contract_address":   collection.ContractAddress,
			"creator":            collection.Creator,
			"collection_type":    collection.CollectionType,
			"max_supply":         collection.MaxSupply.String(),
			"royalty_recipient":  collection.RoyaltyRecipient,
			"royalty_percentage": collection.RoyaltyPercentage,
			"created_at":         collection.CreatedAt,
		},
		Timestamp: time.Now(),
	}

	return s.publisher.PublishDomainEvent(ctx, domainEvent)
}

// publishCollectionUpdatedEvent publishes a collection updated domain event
func (s *CatalogService) publishCollectionUpdatedEvent(ctx context.Context, collection *domain.Collection) error {
	domainEvent := &domain.DomainEvent{
		Schema:      "marketplace.domain.v1",
		Version:     "1.0",
		EventID:     fmt.Sprintf("collection_updated_%s_%d", collection.ID, time.Now().UnixNano()),
		EventType:   "collection_updated",
		AggregateID: collection.ID,
		ChainID:     collection.ChainID,
		Data: map[string]interface{}{
			"id":            collection.ID,
			"slug":          collection.Slug,
			"name":          collection.Name,
			"description":   collection.Description,
			"owner":         collection.Owner,
			"is_verified":   collection.IsVerified,
			"is_explicit":   collection.IsExplicit,
			"is_featured":   collection.IsFeatured,
			"image_url":     collection.ImageURL,
			"banner_url":    collection.BannerURL,
			"external_url":  collection.ExternalURL,
			"discord_url":   collection.DiscordURL,
			"twitter_url":   collection.TwitterURL,
			"instagram_url": collection.InstagramURL,
			"telegram_url":  collection.TelegramURL,
			"floor_price":   collection.FloorPrice.String(),
			"volume_traded": collection.VolumeTraded.String(),
			"updated_at":    collection.UpdatedAt,
		},
		Timestamp: time.Now(),
	}

	return s.publisher.PublishDomainEvent(ctx, domainEvent)
}

// publishCollectionUpsertedEvent publishes a collection upserted domain event (per CREATE.md)
func (s *CatalogService) publishCollectionUpsertedEvent(ctx context.Context, collection *domain.Collection, created bool) error {
	eventType := "collection_upserted"
	if created {
		eventType = "collection_created"
	}

	domainEvent := &domain.DomainEvent{
		Schema:      "marketplace.domain.v1",
		Version:     "1.0",
		EventID:     fmt.Sprintf("%s_%s_%d", eventType, collection.ID, time.Now().UnixNano()),
		EventType:   "collection_upserted", // Always use upserted for routing per CREATE.md
		AggregateID: collection.ID,
		ChainID:     collection.ChainID,
		Data: map[string]interface{}{
			"id":                 collection.ID,
			"slug":               collection.Slug,
			"name":               collection.Name,
			"chain_id":           collection.ChainID,
			"contract_address":   collection.ContractAddress,
			"creator":            collection.Creator,
			"collection_type":    collection.CollectionType,
			"max_supply":         collection.MaxSupply.String(),
			"royalty_recipient":  collection.RoyaltyRecipient,
			"royalty_percentage": collection.RoyaltyPercentage,
			"created_at":         collection.CreatedAt,
			"updated_at":         collection.UpdatedAt,
			"tx_hash":            collection.TxHash, // Extract from event
			"is_new":             created,
		},
		Timestamp: time.Now(),
	}

	return s.publisher.PublishDomainEvent(ctx, domainEvent)
}
