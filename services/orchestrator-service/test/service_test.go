package test

import (
	"context"
	"testing"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/service"
	chainpb "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	protoChainRegistry "github.com/quangdang46/NFT-Marketplace/shared/proto/chainregistry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

// Mock repository for testing
type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) Create(ctx context.Context, it *domain.Intent) error {
	args := m.Called(ctx, it)
	return args.Error(0)
}

func (m *MockRepo) UpdateTxHash(ctx context.Context, intentID string, txHash string, contractAddr *domain.Address) error {
	args := m.Called(ctx, intentID, txHash, contractAddr)
	return args.Error(0)
}

func (m *MockRepo) UpdateStatus(ctx context.Context, intentID string, status domain.IntentStatus, errMsg *string) error {
	args := m.Called(ctx, intentID, status, errMsg)
	return args.Error(0)
}

func (m *MockRepo) GetByID(ctx context.Context, intentID string) (*domain.Intent, error) {
	args := m.Called(ctx, intentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Intent), args.Error(1)
}

func (m *MockRepo) FindByChainTx(ctx context.Context, chainID domain.ChainID, txHash string) (*domain.Intent, error) {
	args := m.Called(ctx, chainID, txHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Intent), args.Error(1)
}

// Mock status cache for testing
type MockStatusCache struct {
	mock.Mock
}

func (m *MockStatusCache) SetIntentStatus(ctx context.Context, payload domain.IntentStatusPayload, ttl time.Duration) error {
	args := m.Called(ctx, payload, ttl)
	return args.Error(0)
}

// Mock chain registry client for testing
type MockChainRegistryClient struct {
	mock.Mock
}

func (m *MockChainRegistryClient) GetContracts(ctx context.Context, req *protoChainRegistry.GetContractsRequest, opts ...grpc.CallOption) (*protoChainRegistry.GetContractsResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*protoChainRegistry.GetContractsResponse), args.Error(1)
}

// Add other required methods for the interface
func (m *MockChainRegistryClient) GetGasPolicy(ctx context.Context, req *protoChainRegistry.GetGasPolicyRequest, opts ...grpc.CallOption) (*protoChainRegistry.GetGasPolicyResponse, error) {
	return nil, nil
}

func (m *MockChainRegistryClient) GetRpcEndpoints(ctx context.Context, req *protoChainRegistry.GetRpcEndpointsRequest, opts ...grpc.CallOption) (*protoChainRegistry.GetRpcEndpointsResponse, error) {
	return nil, nil
}

func (m *MockChainRegistryClient) GetContractMeta(ctx context.Context, req *protoChainRegistry.GetContractMetaRequest, opts ...grpc.CallOption) (*protoChainRegistry.GetContractMetaResponse, error) {
	return nil, nil
}

func (m *MockChainRegistryClient) GetAbiBlob(ctx context.Context, req *protoChainRegistry.GetAbiBlobRequest, opts ...grpc.CallOption) (*protoChainRegistry.GetAbiBlobResponse, error) {
	return nil, nil
}

func (m *MockChainRegistryClient) ResolveProxy(ctx context.Context, req *protoChainRegistry.ResolveProxyRequest, opts ...grpc.CallOption) (*protoChainRegistry.ResolveProxyResponse, error) {
	return nil, nil
}

func (m *MockChainRegistryClient) BumpVersion(ctx context.Context, req *protoChainRegistry.BumpVersionRequest, opts ...grpc.CallOption) (*protoChainRegistry.BumpVersionResponse, error) {
	return nil, nil
}

func (m *MockChainRegistryClient) GetAbiByAddress(ctx context.Context, req *protoChainRegistry.GetAbiByAddressRequest, opts ...grpc.CallOption) (*protoChainRegistry.GetAbiBlobResponse, error) {
	return nil, nil
}

// Mock encoder to avoid ABI dependency
type MockEncoder struct{}

func (m *MockEncoder) EncodeCreateCollection(ctx context.Context, chainID domain.ChainID, factory domain.Address, p domain.PrepareCreateCollectionInput) (domain.Address, []byte, string, *domain.Address, error) {
	to := domain.Address("0x0000000000000000000000000000000000000001")
	data := []byte{0x01}
	value := "0"
	previewAddr := domain.Address("0x0000000000000000000000000000000000000002")
	return to, data, value, &previewAddr, nil
}

func (m *MockEncoder) EncodeMint(ctx context.Context, chainID domain.ChainID, contract domain.Address, standard domain.Standard, p domain.PrepareMintInput) (domain.Address, []byte, string, error) {
	to := domain.Address("0x0000000000000000000000000000000000000003")
	data := []byte{0x02}
	value := "0"
	return to, data, value, nil
}

// Helper function to create service with mocked dependencies
func createTestService(mockRepo *MockRepo, mockStatusCache *MockStatusCache, mockChainRegistry *MockChainRegistryClient) domain.OrchestratorService {
	encoder := &MockEncoder{}
	return service.NewOrchestrator(mockRepo, encoder, mockStatusCache, mockChainRegistry, false)
}

func TestPrepareCreateCollection(t *testing.T) {
	// Arrange
	mockRepo := &MockRepo{}
	mockStatusCache := &MockStatusCache{}
	mockChainRegistry := &MockChainRegistryClient{}

	svc := createTestService(mockRepo, mockStatusCache, mockChainRegistry)

	ctx := context.Background()
	input := domain.PrepareCreateCollectionInput{
		ChainID:  "eip155:8453",
		Name:     "Test Collection",
		Symbol:   "TEST",
		Creator:  "0x1234567890123456789012345678901234567890",
		TokenURI: "ipfs://test",
	}

	// Mock chain-registry response
	factoryContract := &protoChainRegistry.Contract{
		Name:    "CollectionFactory",
		Address: "0x1234567890123456789012345678901234567890",
	}
	chainRegistryResp := &protoChainRegistry.GetContractsResponse{
		ChainId:   "eip155:8453",
		Contracts: []*protoChainRegistry.Contract{factoryContract},
	}

	// Mock expectations
	mockChainRegistry.On("GetContracts", ctx, mock.AnythingOfType("*chainregistry.GetContractsRequest")).Return(chainRegistryResp, nil)
	mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.Intent")).Return(nil)
	mockRepo.On("UpdateTxHash", ctx, mock.AnythingOfType("string"), "", mock.AnythingOfType("*string")).Return(nil)
	mockStatusCache.On("SetIntentStatus", ctx, mock.AnythingOfType("domain.IntentStatusPayload"), domain.DefaultIntentTTL).Return(nil)

	// Act
	result, err := svc.PrepareCreateCollection(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, result.IntentID)
	assert.NotNil(t, result.Tx)
	assert.Equal(t, "eip155:8453", input.ChainID)

	mockRepo.AssertExpectations(t)
	mockStatusCache.AssertExpectations(t)
	mockChainRegistry.AssertExpectations(t)
}

func TestPrepareMint(t *testing.T) {
	// Arrange
	mockRepo := &MockRepo{}
	mockStatusCache := &MockStatusCache{}
	mockChainRegistry := &MockChainRegistryClient{}

	svc := createTestService(mockRepo, mockStatusCache, mockChainRegistry)

	ctx := context.Background()
	input := domain.PrepareMintInput{
		ChainID:  "eip155:8453",
		Contract: "0x1234567890123456789012345678901234567890",
		Standard: domain.StdERC721,
		Minter:   "0xabcdefabcdefabcdefabcdefabcdefabcdefabcd",
		Quantity: 1,
	}

	// Mock expectations
	mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.Intent")).Return(nil)
	mockStatusCache.On("SetIntentStatus", ctx, mock.AnythingOfType("domain.IntentStatusPayload"), domain.DefaultIntentTTL).Return(nil)

	// Act
	result, err := svc.PrepareMint(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, result.IntentID)
	assert.NotNil(t, result.Tx)
	assert.Equal(t, domain.StdERC721, input.Standard)

	mockRepo.AssertExpectations(t)
	mockStatusCache.AssertExpectations(t)
}

func TestTrackTx(t *testing.T) {
	// Arrange
	mockRepo := &MockRepo{}
	mockStatusCache := &MockStatusCache{}
	mockChainRegistry := &MockChainRegistryClient{}

	svc := createTestService(mockRepo, mockStatusCache, mockChainRegistry)

	ctx := context.Background()
	input := domain.TrackTxInput{
		IntentID: "test-intent-id",
		ChainID:  "eip155:8453",
		TxHash:   "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
	}

	existingIntent := &domain.Intent{
		ID:     "test-intent-id",
		Kind:   domain.IntentKindCollection,
		Status: domain.IntentPending,
	}

	// Mock expectations
	mockRepo.On("GetByID", ctx, "test-intent-id").Return(existingIntent, nil)
	mockRepo.On("FindByChainTx", ctx, "eip155:8453", input.TxHash).Return(nil, domain.ErrNotFound)
	mockRepo.On("UpdateTxHash", ctx, "test-intent-id", input.TxHash, (*domain.Address)(nil)).Return(nil)
	mockRepo.On("UpdateStatus", ctx, "test-intent-id", domain.IntentReady, (*string)(nil)).Return(nil)
	mockStatusCache.On("SetIntentStatus", ctx, mock.AnythingOfType("domain.IntentStatusPayload"), domain.DefaultIntentTTL).Return(nil)

	// Act
	ok, err := svc.TrackTx(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.True(t, ok)

	mockRepo.AssertExpectations(t)
	mockStatusCache.AssertExpectations(t)
}

func TestPrepareCreateCollectionWithType(t *testing.T) {
	mockRepo := new(MockRepo)
	mockStatusCache := new(MockStatusCache)
	mockChainRegistry := new(MockChainRegistryClient)
	svc := createTestService(mockRepo, mockStatusCache, mockChainRegistry)

	ctx := context.Background()
	input := domain.PrepareCreateCollectionInput{
		ChainID:  "eip155:1",
		Name:     "Test Collection",
		Symbol:   "TEST",
		Creator:  "0x1234567890123456789012345678901234567890",
		TokenURI: "https://example.com/metadata",
		Type:     domain.StdERC721, // Test with ERC721 type
	}

	// Mock chain registry response
	mockChainRegistry.On("GetContracts", ctx, &protoChainRegistry.GetContractsRequest{
		ChainId: "eip155:1",
	}).Return(&protoChainRegistry.GetContractsResponse{
		Contracts: []*protoChainRegistry.Contract{
			{
				Name:    "ERC721CollectionFactory",
				Address: "0xabcdef1234567890abcdef1234567890abcdef12",
			},
		},
	}, nil)

	// Mock ABI response
	mockChainRegistry.On("GetAbiByAddress", ctx, &chainpb.GetAbiByAddressRequest{
		ChainId: "eip155:1",
		Address: "0xabcdef1234567890abcdef1234567890abcdef12",
	}).Return(&protoChainRegistry.GetAbiBlobResponse{
		AbiJson: `{"abi":[{"type":"function","name":"createERC721Collection","inputs":[{"name":"params","type":"tuple","components":[{"name":"name","type":"string"},{"name":"symbol","type":"string"},{"name":"owner","type":"address"},{"name":"description","type":"string"},{"name":"mintPrice","type":"uint256"},{"name":"royaltyFee","type":"uint256"},{"name":"maxSupply","type":"uint256"},{"name":"mintLimitPerWallet","type":"uint256"},{"name":"mintStartTime","type":"uint256"},{"name":"allowlistMintPrice","type":"uint256"},{"name":"publicMintPrice","type":"uint256"},{"name":"allowlistStageDuration","type":"uint256"},{"name":"tokenURI","type":"string"}]}],"outputs":[{"name":"","type":"address"}],"stateMutability":"nonpayable"}]}`,
	}, nil)

	// Mock repository calls
	mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.Intent")).Return(nil)
	mockRepo.On("UpdateTxHash", ctx, mock.AnythingOfType("string"), "", mock.AnythingOfType("*domain.Address")).Return(nil)
	mockStatusCache.On("SetIntentStatus", ctx, mock.AnythingOfType("domain.IntentStatusPayload"), mock.AnythingOfType("time.Duration")).Return(nil)

	result, err := svc.PrepareCreateCollection(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.IntentID)
	assert.NotNil(t, result.Tx)

	mockRepo.AssertExpectations(t)
	mockStatusCache.AssertExpectations(t)
	mockChainRegistry.AssertExpectations(t)
}

func TestPrepareCreateCollectionWithERC1155Type(t *testing.T) {
	mockRepo := new(MockRepo)
	mockStatusCache := new(MockStatusCache)
	mockChainRegistry := new(MockChainRegistryClient)
	svc := createTestService(mockRepo, mockStatusCache, mockChainRegistry)

	ctx := context.Background()
	input := domain.PrepareCreateCollectionInput{
		ChainID:  "eip155:1",
		Name:     "Test ERC1155 Collection",
		Symbol:   "TEST1155",
		Creator:  "0x1234567890123456789012345678901234567890",
		TokenURI: "https://example.com/metadata/{id}",
		Type:     domain.StdERC1155, // Test with ERC1155 type
	}

	// Mock chain registry response
	mockChainRegistry.On("GetContracts", ctx, &protoChainRegistry.GetContractsRequest{
		ChainId: "eip155:1",
	}).Return(&protoChainRegistry.GetContractsResponse{
		Contracts: []*protoChainRegistry.Contract{
			{
				Name:    "ERC1155CollectionFactory",
				Address: "0xabcdef1234567890abcdef1234567890abcdef12",
			},
		},
	}, nil)

	// Mock ABI response
	mockChainRegistry.On("GetAbiByAddress", ctx, &chainpb.GetAbiByAddressRequest{
		ChainId: "eip155:1",
		Address: "0xabcdef1234567890abcdef1234567890abcdef12",
	}).Return(&protoChainRegistry.GetAbiBlobResponse{
		AbiJson: `{"abi":[{"type":"function","name":"createERC1155Collection","inputs":[{"name":"params","type":"tuple","components":[{"name":"name","type":"string"},{"name":"symbol","type":"string"},{"name":"owner","type":"address"},{"name":"description","type":"string"},{"name":"mintPrice","type":"uint256"},{"name":"royaltyFee","type":"uint256"},{"name":"maxSupply","type":"uint256"},{"name":"mintLimitPerWallet","type":"uint256"},{"name":"mintStartTime","type":"uint256"},{"name":"allowlistMintPrice","type":"uint256"},{"name":"publicMintPrice","type":"uint256"},{"name":"allowlistStageDuration","type":"uint256"},{"name":"tokenURI","type":"string"}]}],"outputs":[{"name":"","type":"address"}],"stateMutability":"nonpayable"}]}`,
	}, nil)

	// Mock repository calls
	mockRepo.On("Create", ctx, mock.AnythingOfType("*domain.Intent")).Return(nil)
	mockRepo.On("UpdateTxHash", ctx, mock.AnythingOfType("string"), "", mock.AnythingOfType("*domain.Address")).Return(nil)
	mockStatusCache.On("SetIntentStatus", ctx, mock.AnythingOfType("domain.IntentStatusPayload"), mock.AnythingOfType("time.Duration")).Return(nil)

	result, err := svc.PrepareCreateCollection(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.IntentID)
	assert.NotNil(t, result.Tx)

	mockRepo.AssertExpectations(t)
	mockStatusCache.AssertExpectations(t)
	mockChainRegistry.AssertExpectations(t)
}
