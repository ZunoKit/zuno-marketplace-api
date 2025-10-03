package test

import (
	"context"
	"testing"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing
type MockCollectionsRepository struct {
	mock.Mock
}

func (m *MockCollectionsRepository) Upsert(ctx context.Context, c domain.Collection) (created bool, err error) {
	args := m.Called(ctx, c)
	return args.Bool(0), args.Error(1)
}

func (m *MockCollectionsRepository) GetByPK(ctx context.Context, chainID domain.ChainID, contract domain.Address) (domain.Collection, error) {
	args := m.Called(ctx, chainID, contract)
	return args.Get(0).(domain.Collection), args.Error(1)
}

type MockProcessedEventsRepository struct {
	mock.Mock
}

func (m *MockProcessedEventsRepository) MarkProcessed(ctx context.Context, eventID string) (bool, error) {
	args := m.Called(ctx, eventID)
	return args.Bool(0), args.Error(1)
}

type MockMessagePublisher struct {
	mock.Mock
}

func (m *MockMessagePublisher) PublishCollectionUpserted(ctx context.Context, collection *domain.Collection) error {
	args := m.Called(ctx, collection)
	return args.Error(0)
}

func (m *MockMessagePublisher) PublishDomainEvent(ctx context.Context, event *domain.DomainEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func TestCatalogService_HandleCollectionCreated(t *testing.T) {
	// Arrange
	mockCollectionRepo := new(MockCollectionsRepository)
	mockProcessedEventRepo := new(MockProcessedEventsRepository)
	mockPublisher := new(MockMessagePublisher)

	service := service.NewCatalogService(mockCollectionRepo, mockProcessedEventRepo, mockPublisher)

	ctx := context.Background()
	event := &domain.CollectionEvent{
		EventID:   "test-event-123",
		EventType: "collection_created",
		ChainID:   "eip155-1",
		Contract:  "0x1234567890123456789012345678901234567890",
		Data: map[string]interface{}{
			"collection_address": "0x1234567890123456789012345678901234567890",
			"creator":            "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			"name":               "Test Collection",
			"collection_type":    "ERC721",
			"description":        "A test collection",
			"max_supply":         "10000",
			"royalty_recipient":  "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			"royalty_percentage": "500",
		},
		Timestamp: time.Now(),
	}

	// Mock expectations
	mockProcessedEventRepo.On("MarkProcessed", ctx, event.EventID).Return(true, nil)
	mockCollectionRepo.On("Upsert", ctx, mock.AnythingOfType("domain.Collection")).Return(true, nil)
	mockPublisher.On("PublishDomainEvent", ctx, mock.AnythingOfType("*domain.DomainEvent")).Return(nil)

	// Act
	err := service.HandleCollectionCreated(ctx, event)

	// Assert
	assert.NoError(t, err)
	mockProcessedEventRepo.AssertExpectations(t)
	mockCollectionRepo.AssertExpectations(t)
	mockPublisher.AssertExpectations(t)
}

func TestCatalogService_HandleCollectionCreated_AlreadyProcessed(t *testing.T) {
	// Arrange
	mockCollectionRepo := new(MockCollectionsRepository)
	mockProcessedEventRepo := new(MockProcessedEventsRepository)
	mockPublisher := new(MockMessagePublisher)

	service := service.NewCatalogService(mockCollectionRepo, mockProcessedEventRepo, mockPublisher)

	ctx := context.Background()
	event := &domain.CollectionEvent{
		EventID:   "test-event-123",
		EventType: "collection_created",
		ChainID:   "eip155-1",
		Contract:  "0x1234567890123456789012345678901234567890",
		Data: map[string]interface{}{
			"collection_address": "0x1234567890123456789012345678901234567890",
			"creator":            "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
			"name":               "Test Collection",
			"collection_type":    "ERC721",
		},
		Timestamp: time.Now(),
	}

	// Mock expectations - event already processed
	mockProcessedEventRepo.On("MarkProcessed", ctx, event.EventID).Return(false, nil)

	// Act
	err := service.HandleCollectionCreated(ctx, event)

	// Assert
	assert.NoError(t, err)
	mockProcessedEventRepo.AssertExpectations(t)
	mockCollectionRepo.AssertNotCalled(t, "Upsert")
	mockPublisher.AssertNotCalled(t, "PublishDomainEvent")
}
