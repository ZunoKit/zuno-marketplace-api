package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	protoChainRegistry "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"google.golang.org/grpc/metadata"
)

type Service struct {
	repo          domain.OrchestratorRepo
	encoder       domain.Encoder
	statusCache   domain.StatusCache
	chainRegistry protoChainRegistry.ChainRegistryServiceClient
	// feature flags
	sessionLinkedIntents     bool
	sessionValidationTimeout time.Duration
}

// NewOrchestrator preserves the original 5-arg constructor used in tests
func NewOrchestrator(
    repo domain.OrchestratorRepo,
    encoder domain.Encoder,
    statusCache domain.StatusCache,
    chainRegistry protoChainRegistry.ChainRegistryServiceClient,
    sessionLinkedIntents bool,
) domain.OrchestratorService {
    return NewOrchestratorWithTimeout(
        repo,
        encoder,
        statusCache,
        chainRegistry,
        sessionLinkedIntents,
        2*time.Second,
    )
}

// NewOrchestratorWithTimeout allows specifying the session validation timeout
func NewOrchestratorWithTimeout(
    repo domain.OrchestratorRepo,
    encoder domain.Encoder,
    statusCache domain.StatusCache,
    chainRegistry protoChainRegistry.ChainRegistryServiceClient,
    sessionLinkedIntents bool,
    sessionValidationTimeout time.Duration,
) domain.OrchestratorService {
    if sessionValidationTimeout == 0 {
        sessionValidationTimeout = 2 * time.Second
    }
    return &Service{
        repo:                     repo,
        encoder:                  encoder,
        statusCache:              statusCache,
        chainRegistry:            chainRegistry,
        sessionLinkedIntents:     sessionLinkedIntents,
        sessionValidationTimeout: sessionValidationTimeout,
    }
}

func (s *Service) getFactoryAddressAndType(ctx context.Context, chainID domain.ChainID, in domain.PrepareCreateCollectionInput) (domain.Address, domain.Standard, error) {
	req := &protoChainRegistry.GetContractsRequest{
		ChainId: chainID,
	}

	resp, err := s.chainRegistry.GetContracts(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("get contracts from chain-registry: %w", err)
	}

	collectionType := in.Type

	var targetFactory *protoChainRegistry.Contract
	switch collectionType {
	case domain.StdERC721:
		for _, contract := range resp.Contracts {
			if contract.Name == "ERC721CollectionFactory" {
				targetFactory = contract
				break
			}
		}
	case domain.StdERC1155:
		for _, contract := range resp.Contracts {
			if contract.Name == "ERC1155CollectionFactory" {
				targetFactory = contract
				break
			}
		}
	default:
		return "", "", fmt.Errorf("unsupported collection type: %s", collectionType)
	}

	if targetFactory == nil {
		availableContracts := make([]string, 0, len(resp.Contracts))
		for _, contract := range resp.Contracts {
			availableContracts = append(availableContracts, contract.Name)
		}
		return "", "", fmt.Errorf("factory for collection type %s not found for chain %s. Available contracts: %v", collectionType, chainID, availableContracts)
	}

	return domain.Address(targetFactory.Address), collectionType, nil
}

func (s *Service) PrepareCreateCollection(ctx context.Context, in domain.PrepareCreateCollectionInput) (*domain.PrepareCreateCollectionResult, error) {
	if err := ValidateCreateCollectionInput(in); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	factoryAddr, collectionType, err := s.getFactoryAddressAndType(ctx, in.ChainID, in)
	fmt.Println("factoryAddr", factoryAddr)
	fmt.Println("collectionType", collectionType)
	if err != nil {
		return nil, fmt.Errorf("get factory address: %w", err)
	}

	intentID := uuid.New().String()
	now := time.Now()

	intent := &domain.Intent{
		ID:        intentID,
		Kind:      domain.IntentKindCollection,
		ChainID:   in.ChainID,
		Status:    domain.IntentPending,
		CreatedBy: in.CreatedBy,
		ReqPayloadJSON: map[string]interface{}{
			"input":          in,
			"collectionType": collectionType,
			"factoryAddress": factoryAddr,
		},
		DeadlineAt: in.DeadlineAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Feature-flagged session validation and correlation
	if s.sessionLinkedIntents {
		vctx, cancel := context.WithTimeout(ctx, s.sessionValidationTimeout)
		defer cancel()
		var sessionID string
		if md, ok := metadata.FromIncomingContext(vctx); ok {
			vals := md.Get("x-auth-session-id")
			if len(vals) > 0 {
				sessionID = vals[0]
			}
		}
		if sessionID == "" {
			return nil, domain.ErrUnauthenticated
		}
		if _, err := uuid.Parse(sessionID); err != nil {
			return nil, domain.ErrUnauthenticated
		}
		intent.AuthSessionID = &sessionID
		log.Printf("audit|event=intent_validate|intent_id=%s|session_id=%s|timestamp=%s", intentID, sessionID, now.UTC().Format(time.RFC3339Nano))
	} else {
		// Best-effort correlation without enforcement
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			vals := md.Get("x-auth-session-id")
			if len(vals) > 0 && vals[0] != "" {
				sid := vals[0]
				intent.AuthSessionID = &sid
				log.Printf("audit|event=intent_create|intent_id=%s|session_id=%s|timestamp=%s", intentID, sid, now.UTC().Format(time.RFC3339Nano))
			}
		}
	}

	if err := s.repo.Create(ctx, intent); err != nil {
		return nil, fmt.Errorf("create intent: %w", err)
	}
	if intent.AuthSessionID != nil {
		_ = s.repo.InsertSessionIntentAudit(ctx, *intent.AuthSessionID, intentID, in.CreatedBy, map[string]any{
			"operation":      "collection_creation",
			"chainId":        in.ChainID,
			"factoryAddress": factoryAddr,
			"collectionType": collectionType,
			"requestedAt":    now.UTC().Format(time.RFC3339Nano),
		})
	}

	to, data, value, preview, err := s.encoder.EncodeCreateCollection(ctx, in.ChainID, factoryAddr, in)
	if err != nil {
		errMsg := err.Error()
		if updateErr := s.repo.UpdateStatus(ctx, intentID, domain.IntentFailed, &errMsg); updateErr != nil {
			fmt.Printf("Failed to update intent status to failed: %v", updateErr)
		}
		return nil, fmt.Errorf("encode create collection: %w", err)
	}

	if preview != nil {
		intent.PreviewAddress = preview
		if err := s.repo.UpdateTxHash(ctx, intentID, "", preview); err != nil {
			fmt.Printf("Failed to update intent preview address: %v", err)
		}
	}

	txRequest := domain.TxRequest{
		To:             to,
		Data:           data,
		Value:          value,
		PreviewAddress: preview,
	}

	statusPayload := domain.IntentStatusPayload{
		IntentID:        intentID,
		Kind:            domain.IntentKindCollection,
		Status:          domain.IntentPending,
		ChainID:         &in.ChainID,
		ContractAddress: preview,
	}
	s.statusCache.SetIntentStatus(ctx, statusPayload, domain.DefaultIntentTTL)

	return &domain.PrepareCreateCollectionResult{
		IntentID: intentID,
		Tx:       txRequest,
	}, nil
}

func (s *Service) PrepareMint(ctx context.Context, in domain.PrepareMintInput) (*domain.PrepareMintResult, error) {
	if in.ChainID == "" || in.Contract == "" || in.Minter == "" {
		return nil, domain.ErrInvalidInput
	}

	switch in.Standard {
	case domain.StdERC721, domain.StdERC1155:
	default:
		return nil, domain.ErrUnsupportedStd
	}

	intentID := uuid.New().String()
	now := time.Now()

	intent := &domain.Intent{
		ID:             intentID,
		Kind:           domain.IntentKindMint,
		ChainID:        in.ChainID,
		Status:         domain.IntentPending,
		CreatedBy:      in.CreatedBy,
		ReqPayloadJSON: in,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.Create(ctx, intent); err != nil {
		return nil, fmt.Errorf("create intent: %w", err)
	}

	to, data, value, err := s.encoder.EncodeMint(ctx, in.ChainID, in.Contract, in.Standard, in)
	if err != nil {
		errMsg := err.Error()
		s.repo.UpdateStatus(ctx, intentID, domain.IntentFailed, &errMsg)
		return nil, fmt.Errorf("encode mint: %w", err)
	}

	txRequest := domain.TxRequest{
		To:    to,
		Data:  data,
		Value: value,
	}

	statusPayload := domain.IntentStatusPayload{
		IntentID: intentID,
		Kind:     domain.IntentKindMint,
		Status:   domain.IntentPending,
		ChainID:  &in.ChainID,
	}
	s.statusCache.SetIntentStatus(ctx, statusPayload, domain.DefaultIntentTTL)

	return &domain.PrepareMintResult{
		IntentID: intentID,
		Tx:       txRequest,
	}, nil
}

func (s *Service) TrackTx(ctx context.Context, in domain.TrackTxInput) (ok bool, err error) {
	if in.IntentID == "" || in.ChainID == "" || in.TxHash == "" {
		return false, domain.ErrInvalidInput
	}

	intent, err := s.repo.GetByID(ctx, in.IntentID)
	if err != nil {
		return false, fmt.Errorf("get intent: %w", err)
	}

	if intent.TxHash != nil && *intent.TxHash == in.TxHash {
		return true, nil
	}

	existingIntent, err := s.repo.FindByChainTx(ctx, in.ChainID, in.TxHash)
	if err == nil && existingIntent != nil && existingIntent.ID != in.IntentID {
		return false, domain.ErrDuplicateTx
	}

	err = s.repo.UpdateTxHash(ctx, in.IntentID, in.TxHash, in.Contract)
	if err != nil {
		return false, fmt.Errorf("update tx hash: %w", err)
	}

	err = s.repo.UpdateStatus(ctx, in.IntentID, domain.IntentPending, nil)
	if err != nil {
		return false, fmt.Errorf("update status: %w", err)
	}

	statusPayload := domain.IntentStatusPayload{
		IntentID:        in.IntentID,
		Kind:            intent.Kind,
		Status:          domain.IntentPending,
		ChainID:         &in.ChainID,
		TxHash:          &in.TxHash,
		ContractAddress: in.Contract,
	}
	s.statusCache.SetIntentStatus(ctx, statusPayload, domain.DefaultIntentTTL)

	return true, nil
}

func (s *Service) GetIntentStatus(ctx context.Context, intentID string) (*domain.IntentStatusPayload, error) {
	if intentID == "" {
		return nil, domain.ErrInvalidInput
	}

	intent, err := s.repo.GetByID(ctx, intentID)
	if err != nil {
		return nil, fmt.Errorf("get intent: %w", err)
	}

	statusPayload := domain.IntentStatusPayload{
		IntentID:        intent.ID,
		Kind:            intent.Kind,
		Status:          intent.Status,
		ChainID:         &intent.ChainID,
		TxHash:          intent.TxHash,
		ContractAddress: intent.PreviewAddress,
	}

	return &statusPayload, nil
}
