package repository

import (
	"context"

	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
)

type Tx struct {
	collectionsRepo     domain.CollectionsRepository
	processedEventsRepo domain.ProcessedEventsRepository
}

func NewTx(collectionsRepo domain.CollectionsRepository, processedEventsRepo domain.ProcessedEventsRepository) domain.Tx {
	return &Tx{
		collectionsRepo:     collectionsRepo,
		processedEventsRepo: processedEventsRepo,
	}
}

func (t *Tx) CollectionsRepo() domain.CollectionsRepository {
	return t.collectionsRepo
}

func (t *Tx) ProcessedRepo() domain.ProcessedEventsRepository {
	return t.processedEventsRepo
}

type UnitOfWork struct {
	collectionsRepo     domain.CollectionsRepository
	processedEventsRepo domain.ProcessedEventsRepository
}

func NewUnitOfWork(collectionsRepo domain.CollectionsRepository, processedEventsRepo domain.ProcessedEventsRepository) domain.UnitOfWork {
	return &UnitOfWork{
		collectionsRepo:     collectionsRepo,
		processedEventsRepo: processedEventsRepo,
	}
}

func (uow *UnitOfWork) WithinTx(ctx context.Context, fn func(ctx context.Context, tx domain.Tx) error) error {
	tx := NewTx(uow.collectionsRepo, uow.processedEventsRepo)
	return fn(ctx, tx)
}
