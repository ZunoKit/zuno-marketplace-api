package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	sharedredis "github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type Repo struct {
	pg    *postgres.Postgres
	redis *sharedredis.Redis
}

func NewOrchestratorRepo(pg *postgres.Postgres, redis *sharedredis.Redis) domain.OrchestratorRepo {
	return &Repo{pg: pg, redis: redis}
}

func (r *Repo) Create(ctx context.Context, it *domain.Intent) error {

	reqPayloadJSON, err := json.Marshal(it.ReqPayloadJSON)
	if err != nil {
		return fmt.Errorf("marshal req payload: %w", err)
	}

	_, err = r.pg.GetClient().ExecContext(ctx, CreateIntentQuery,
		it.ID, it.Kind, it.ChainID, it.PreviewAddress, it.TxHash, it.Status,
		it.CreatedBy, reqPayloadJSON, it.Error, it.DeadlineAt, it.CreatedAt, it.UpdatedAt, it.AuthSessionID,
	)
	if err != nil {
		return fmt.Errorf("insert intent: %w", err)
	}

	return nil
}

func (r *Repo) UpdateTxHash(ctx context.Context, intentID string, txHash string, contractAddr *domain.Address) error {
	_, err := r.pg.GetClient().ExecContext(ctx, UpdateTxHashQuery, txHash, contractAddr, time.Now(), intentID)
	if err != nil {
		return fmt.Errorf("update tx hash: %w", err)
	}

	return nil
}

func (r *Repo) UpdateStatus(ctx context.Context, intentID string, status domain.IntentStatus, errMsg *string) error {
	_, err := r.pg.GetClient().ExecContext(ctx, UpdateStatusQuery, status, errMsg, time.Now(), intentID)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	return nil
}

func (r *Repo) GetByID(ctx context.Context, intentID string) (*domain.Intent, error) {
	var it domain.Intent
	var reqPayloadJSON []byte

	err := r.pg.GetClient().QueryRowContext(ctx, GetByIDQuery, intentID).Scan(
		&it.ID, &it.Kind, &it.ChainID, &it.PreviewAddress, &it.TxHash, &it.Status,
		&it.CreatedBy, &reqPayloadJSON, &it.Error, &it.DeadlineAt, &it.CreatedAt, &it.UpdatedAt, &it.AuthSessionID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get intent: %w", err)
	}

	if len(reqPayloadJSON) > 0 {
		err = json.Unmarshal(reqPayloadJSON, &it.ReqPayloadJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal req payload: %w", err)
		}
	}

	return &it, nil
}

// FindByChainTx finds intent by chain ID and transaction hash
func (r *Repo) FindByChainTx(ctx context.Context, chainID domain.ChainID, txHash string) (*domain.Intent, error) {

	var it domain.Intent
	var reqPayloadJSON []byte

	err := r.pg.GetClient().QueryRowContext(ctx, FindByChainTxQuery, chainID, txHash).Scan(
		&it.ID, &it.Kind, &it.ChainID, &it.PreviewAddress, &it.TxHash, &it.Status,
		&it.CreatedBy, &reqPayloadJSON, &it.Error, &it.DeadlineAt, &it.CreatedAt, &it.UpdatedAt, &it.AuthSessionID,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("find by chain tx: %w", err)
	}

	if len(reqPayloadJSON) > 0 {
		err = json.Unmarshal(reqPayloadJSON, &it.ReqPayloadJSON)
		if err != nil {
			return nil, fmt.Errorf("unmarshal req payload: %w", err)
		}
	}

	return &it, nil
}

func (r *Repo) InsertSessionIntentAudit(ctx context.Context, sessionID string, intentID string, userID *string, auditData any) error {
    payload, err := json.Marshal(auditData)
    if err != nil {
        return fmt.Errorf("marshal audit: %w", err)
    }
    _, err = r.pg.GetClient().ExecContext(ctx, InsertSessionIntentAuditQuery, sessionID, intentID, userID, payload)
    if err != nil {
        return fmt.Errorf("insert session_intent_audit: %w", err)
    }
    return nil
}
