package test

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/encode"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/service"
	chainpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// Mock implementations for testing
type mockRepo struct {
	intents map[string]*domain.Intent
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		intents: make(map[string]*domain.Intent),
	}
}

func (r *mockRepo) Create(ctx context.Context, intent *domain.Intent) error {
	r.intents[intent.ID] = intent
	return nil
}

func (r *mockRepo) GetByID(ctx context.Context, id string) (*domain.Intent, error) {
	intent, ok := r.intents[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return intent, nil
}

func (r *mockRepo) FindByChainTx(ctx context.Context, chainID domain.ChainID, txHash string) (*domain.Intent, error) {
	for _, intent := range r.intents {
		if intent.ChainID == chainID && intent.TxHash != nil && *intent.TxHash == txHash {
			return intent, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *mockRepo) UpdateStatus(ctx context.Context, id string, status domain.IntentStatus, errorMsg *string) error {
	intent, ok := r.intents[id]
	if !ok {
		return domain.ErrNotFound
	}
	intent.Status = status
	intent.Error = errorMsg
	intent.UpdatedAt = time.Now()
	return nil
}

func (r *mockRepo) UpdateTxHash(ctx context.Context, id string, txHash string, contract *domain.Address) error {
	intent, ok := r.intents[id]
	if !ok {
		return domain.ErrNotFound
	}
	intent.TxHash = &txHash
	if contract != nil {
		intent.PreviewAddress = contract
	}
	intent.UpdatedAt = time.Now()
	return nil
}

func (r *mockRepo) InsertSessionIntentAudit(ctx context.Context, sessionID string, intentID string, userID *string, auditData any) error {
	// For testing, just validate the parameters
	if sessionID == "" || intentID == "" {
		return domain.ErrInvalidInput
	}
	return nil
}

type mockStatusCache struct {
	cache map[string]domain.IntentStatusPayload
}

func newMockStatusCache() *mockStatusCache {
	return &mockStatusCache{
		cache: make(map[string]domain.IntentStatusPayload),
	}
}

func (c *mockStatusCache) SetIntentStatus(ctx context.Context, payload domain.IntentStatusPayload, ttl time.Duration) error {
	c.cache[payload.IntentID] = payload
	return nil
}

func (c *mockStatusCache) GetIntentStatus(ctx context.Context, intentID string) (*domain.IntentStatusPayload, error) {
	payload, ok := c.cache[intentID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &payload, nil
}

// Integration test for ERC721 minting flow
func TestIntegration_MintERC721_CompleteFlow(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockChainRegistry := NewMockChainRegistryServiceClient(ctrl)
	mockRepo := newMockRepo()
	mockCache := newMockStatusCache()
	encoder := encode.NewEncoder(mockChainRegistry)

	// Create service
	svc := service.NewOrchestrator(
		mockRepo,
		encoder,
		mockCache,
		mockChainRegistry,
		false, // session linked intents disabled
	)

	// Test data
	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI response
	erc721ABI := `{
		"abi": [
			{
				"inputs": [{"name": "to", "type": "address"}],
				"name": "mint",
				"outputs": [],
				"stateMutability": "payable",
				"type": "function"
			}
		]
	}`

	// Setup expectations for PrepareMint
	mockChainRegistry.EXPECT().
		GetAbiByAddress(ctx, &chainpb.GetAbiByAddressRequest{
			ChainId: string(chainID),
			Address: string(contract),
		}).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc721ABI,
		}, nil)

	// No need to mock GetGasPolicy anymore since we're using default value

	// Step 1: Prepare mint
	mintInput := domain.PrepareMintInput{
		ChainID:   chainID,
		Contract:  contract,
		Minter:    minter,
		Standard:  domain.StdERC721,
		Quantity:  1,
		CreatedBy: func(s string) *string { v := string(s); return &v }(string(minter)),
	}

	result, err := svc.PrepareMint(ctx, mintInput)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.IntentID)
	assert.Equal(t, contract, result.Tx.To)
	assert.NotEmpty(t, result.Tx.Data)
	assert.Equal(t, "0", result.Tx.Value)

	// Verify intent was created
	intent, err := mockRepo.GetByID(ctx, result.IntentID)
	require.NoError(t, err)
	assert.Equal(t, domain.IntentKindMint, intent.Kind)
	assert.Equal(t, domain.IntentPending, intent.Status)
	assert.Equal(t, chainID, intent.ChainID)

	// Verify status cache was updated
	status, err := mockCache.GetIntentStatus(ctx, result.IntentID)
	require.NoError(t, err)
	assert.Equal(t, domain.IntentPending, status.Status)

	// Step 2: Track transaction
	txHash := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	trackInput := domain.TrackTxInput{
		IntentID: result.IntentID,
		ChainID:  chainID,
		TxHash:   txHash,
		Contract: &contract,
	}

	ok, err := svc.TrackTx(ctx, trackInput)
	require.NoError(t, err)
	assert.True(t, ok)

	// Verify intent was updated with tx hash
	intent, err = mockRepo.GetByID(ctx, result.IntentID)
	require.NoError(t, err)
	assert.NotNil(t, intent.TxHash)
	assert.Equal(t, txHash, *intent.TxHash)

	// Step 3: Get intent status
	statusResult, err := svc.GetIntentStatus(ctx, result.IntentID)
	require.NoError(t, err)
	assert.Equal(t, result.IntentID, statusResult.IntentID)
	assert.Equal(t, domain.IntentKindMint, statusResult.Kind)
	assert.Equal(t, domain.IntentPending, statusResult.Status)
	assert.Equal(t, &chainID, statusResult.ChainID)
	assert.Equal(t, &txHash, statusResult.TxHash)
}

// Integration test for ERC1155 minting flow
func TestIntegration_MintERC1155_CompleteFlow(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockChainRegistry := NewMockChainRegistryServiceClient(ctrl)
	mockRepo := newMockRepo()
	mockCache := newMockStatusCache()
	encoder := encode.NewEncoder(mockChainRegistry)

	// Create service
	svc := service.NewOrchestrator(
		mockRepo,
		encoder,
		mockCache,
		mockChainRegistry,
		false,
	)

	// Test data
	chainID := domain.ChainID("eip155:137") // Polygon
	contract := domain.Address("0x1155Contract1234567890123456789012345678")
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI response
	erc1155ABI := `{
		"abi": [
			{
				"inputs": [
					{"name": "to", "type": "address"},
					{"name": "id", "type": "uint256"},
					{"name": "amount", "type": "uint256"},
					{"name": "data", "type": "bytes"}
				],
				"name": "mint",
				"outputs": [],
				"stateMutability": "payable",
				"type": "function"
			}
		]
	}`

	// Setup expectations
	mockChainRegistry.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc1155ABI,
		}, nil)

	// No need to mock GetGasPolicy anymore since we're using default value

	// Prepare mint
	mintInput := domain.PrepareMintInput{
		ChainID:   chainID,
		Contract:  contract,
		Minter:    minter,
		Standard:  domain.StdERC1155,
		Quantity:  10, // Mint 10 tokens
		CreatedBy: func(s string) *string { v := string(s); return &v }(string(minter)),
	}

	result, err := svc.PrepareMint(ctx, mintInput)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.IntentID)
	assert.Equal(t, contract, result.Tx.To)
	assert.NotEmpty(t, result.Tx.Data)
	assert.Equal(t, "0", result.Tx.Value)

	// Track transaction
	txHash := "0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	trackInput := domain.TrackTxInput{
		IntentID: result.IntentID,
		ChainID:  chainID,
		TxHash:   txHash,
		Contract: &contract,
	}

	ok, err := svc.TrackTx(ctx, trackInput)
	require.NoError(t, err)
	assert.True(t, ok)
}

// Test minting with session validation enabled
func TestIntegration_MintWithSessionValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockChainRegistry := NewMockChainRegistryServiceClient(ctrl)
	mockRepo := newMockRepo()
	mockCache := newMockStatusCache()
	encoder := encode.NewEncoder(mockChainRegistry)

	// Create service with session validation enabled
	svc := service.NewOrchestratorWithTimeout(
		mockRepo,
		encoder,
		mockCache,
		mockChainRegistry,
		true, // session linked intents enabled
		2*time.Second,
	)

	// Test data
	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	minter := domain.Address("0xUser123456789012345678901234567890123456")
	sessionID := uuid.New().String()

	// Create context with session metadata
	md := metadata.New(map[string]string{
		"x-auth-session-id": sessionID,
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	// Mock ABI response
	erc721ABI := `{
		"abi": [
			{
				"inputs": [{"name": "to", "type": "address"}],
				"name": "mint",
				"outputs": [],
				"type": "function"
			}
		]
	}`

	mockChainRegistry.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc721ABI,
		}, nil)

	// No need to mock GetGasPolicy anymore

	// Prepare mint with session
	mintInput := domain.PrepareMintInput{
		ChainID:   chainID,
		Contract:  contract,
		Minter:    minter,
		Standard:  domain.StdERC721,
		Quantity:  1,
		CreatedBy: func(s string) *string { v := string(s); return &v }(string(minter)),
	}

	result, err := svc.PrepareMint(ctx, mintInput)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify intent has session ID
	intent, err := mockRepo.GetByID(ctx, result.IntentID)
	require.NoError(t, err)
	assert.NotNil(t, intent.AuthSessionID)
	assert.Equal(t, sessionID, *intent.AuthSessionID)
}

// Test minting without session when validation is enabled
func TestIntegration_MintWithoutSession_ShouldFail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockChainRegistry := NewMockChainRegistryServiceClient(ctrl)
	mockRepo := newMockRepo()
	mockCache := newMockStatusCache()
	encoder := encode.NewEncoder(mockChainRegistry)

	// Create service with session validation enabled
	svc := service.NewOrchestrator(
		mockRepo,
		encoder,
		mockCache,
		mockChainRegistry,
		true, // session linked intents enabled
	)

	// Context without session metadata
	ctx := context.Background()

	// Test data
	chainID := domain.ChainID("eip155:1")
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")
	minter := domain.Address("0xUser123456789012345678901234567890123456")

	// Mock ABI response
	erc721ABI := `{
		"abi": [
			{
				"inputs": [{"name": "to", "type": "address"}],
				"name": "mint",
				"outputs": [],
				"type": "function"
			}
		]
	}`

	mockChainRegistry.EXPECT().
		GetAbiByAddress(ctx, gomock.Any()).
		Return(&chainpb.GetAbiBlobResponse{
			AbiJson: erc721ABI,
		}, nil).AnyTimes()

	// No need to mock GetGasPolicy anymore

	// Attempt to prepare mint without session
	mintInput := domain.PrepareMintInput{
		ChainID:   chainID,
		Contract:  contract,
		Minter:    minter,
		Standard:  domain.StdERC721,
		Quantity:  1,
		CreatedBy: func(s string) *string { v := string(s); return &v }(string(minter)),
	}

	_, err := svc.PrepareMint(ctx, mintInput)

	// Should fail with unauthenticated error
	assert.Error(t, err)
	assert.Equal(t, domain.ErrUnauthenticated, err)
}

// Test duplicate transaction tracking
func TestIntegration_DuplicateTransactionTracking(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Setup mocks
	mockChainRegistry := NewMockChainRegistryServiceClient(ctrl)
	mockRepo := newMockRepo()
	mockCache := newMockStatusCache()
	encoder := encode.NewEncoder(mockChainRegistry)

	// Create service
	svc := service.NewOrchestrator(
		mockRepo,
		encoder,
		mockCache,
		mockChainRegistry,
		false,
	)

	// Create two intents
	intent1 := &domain.Intent{
		ID:        uuid.New().String(),
		Kind:      domain.IntentKindMint,
		ChainID:   domain.ChainID("eip155:1"),
		Status:    domain.IntentPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	intent2 := &domain.Intent{
		ID:        uuid.New().String(),
		Kind:      domain.IntentKindMint,
		ChainID:   domain.ChainID("eip155:1"),
		Status:    domain.IntentPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockRepo.Create(ctx, intent1)
	mockRepo.Create(ctx, intent2)

	// Track same transaction for first intent
	txHash := "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	contract := domain.Address("0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb2")

	trackInput1 := domain.TrackTxInput{
		IntentID: intent1.ID,
		ChainID:  domain.ChainID("eip155:1"),
		TxHash:   txHash,
		Contract: &contract,
	}

	ok, err := svc.TrackTx(ctx, trackInput1)
	require.NoError(t, err)
	assert.True(t, ok)

	// Try to track same transaction for second intent
	trackInput2 := domain.TrackTxInput{
		IntentID: intent2.ID,
		ChainID:  domain.ChainID("eip155:1"),
		TxHash:   txHash,
		Contract: &contract,
	}

	ok, err = svc.TrackTx(ctx, trackInput2)
	assert.Error(t, err)
	assert.Equal(t, domain.ErrDuplicateTx, err)
	assert.False(t, ok)
}
