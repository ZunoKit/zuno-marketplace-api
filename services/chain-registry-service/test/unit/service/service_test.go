package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository implements domain.ChainRegistryRepository for testing
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetContracts(ctx context.Context, chainID domain.ChainID) (*domain.ChainContracts, error) {
	args := m.Called(ctx, chainID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ChainContracts), args.Error(1)
}

func (m *MockRepository) GetGasPolicy(ctx context.Context, chainID domain.ChainID) (*domain.ChainGasPolicy, error) {
	args := m.Called(ctx, chainID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ChainGasPolicy), args.Error(1)
}

func (m *MockRepository) GetRpcEndpoints(ctx context.Context, chainID domain.ChainID) (*domain.ChainRpcEndpoints, error) {
	args := m.Called(ctx, chainID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ChainRpcEndpoints), args.Error(1)
}

func (m *MockRepository) BumpVersion(ctx context.Context, chainID domain.ChainID, reason string) (newVersion string, err error) {
	args := m.Called(ctx, chainID, reason)
	return args.String(0), args.Error(1)
}

func (m *MockRepository) GetContractMeta(ctx context.Context, chainID domain.ChainID, address domain.Address) (*domain.ContractMeta, error) {
	args := m.Called(ctx, chainID, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ContractMeta), args.Error(1)
}

func (m *MockRepository) GetAbiBlob(ctx context.Context, sha domain.Sha256) (abiJSON []byte, etag string, err error) {
	args := m.Called(ctx, sha)
	var blob []byte
	if v := args.Get(0); v != nil {
		if b, ok := v.([]byte); ok {
			blob = b
		}
	}
	return blob, args.String(1), args.Error(2)
}

func (m *MockRepository) ResolveProxy(ctx context.Context, chainID domain.ChainID, address domain.Address) (implAddress domain.Address, abiSha256 domain.Sha256, err error) {
	args := m.Called(ctx, chainID, address)
	return args.String(0), args.String(1), args.Error(2)
}

func TestService_GetContracts(t *testing.T) {
	tests := []struct {
		name        string
		chainID     domain.ChainID
		setupMock   func(*MockRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name:    "successful get contracts",
			chainID: "eip155:1",
			setupMock: func(mockRepo *MockRepository) {
				expected := &domain.ChainContracts{
					ChainID:         "eip155:1",
					ChainNumeric:    1,
					NativeSymbol:    "ETH",
					Contracts:       []domain.Contract{},
					Params:          domain.ChainParams{RequiredConfirmations: 12, ReorgDepth: 12, BlockTimeMs: 12000},
					RegistryVersion: "1.0.0",
				}
				mockRepo.On("GetContracts", mock.Anything, "eip155:1").Return(expected, nil)
			},
			expectError: false,
		},
		{
			name:    "empty chain ID",
			chainID: "",
			setupMock: func(mockRepo *MockRepository) {
				// No mock setup needed as validation should fail first
			},
			expectError: true,
			errorMsg:    "chain_id is required",
		},
		{
			name:    "repository error",
			chainID: "eip155:1",
			setupMock: func(mockRepo *MockRepository) {
				mockRepo.On("GetContracts", mock.Anything, "eip155:1").Return(nil, assert.AnError)
			},
			expectError: true,
			errorMsg:    "failed to get contracts from repository",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			tt.setupMock(mockRepo)

			svc := service.New(mockRepo)
			result, err := svc.GetContracts(context.Background(), tt.chainID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.chainID, result.ChainID)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetGasPolicy(t *testing.T) {
	tests := []struct {
		name        string
		chainID     domain.ChainID
		setupMock   func(*MockRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name:    "successful get gas policy",
			chainID: "eip155:1",
			setupMock: func(mockRepo *MockRepository) {
				expected := &domain.ChainGasPolicy{
					ChainID: "eip155:1",
					Policy: domain.GasPolicy{
						MaxFeeGwei:              50.0,
						PriorityFeeGwei:         2.0,
						Multiplier:              1.1,
						LastObservedBaseFeeGwei: 20.0,
						UpdatedAt:               time.Now(),
					},
					RegistryVersion: "1.0.0",
				}
				mockRepo.On("GetGasPolicy", mock.Anything, "eip155:1").Return(expected, nil)
			},
			expectError: false,
		},
		{
			name:    "empty chain ID",
			chainID: "",
			setupMock: func(mockRepo *MockRepository) {
				// No mock setup needed as validation should fail first
			},
			expectError: true,
			errorMsg:    "chain_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			tt.setupMock(mockRepo)

			svc := service.New(mockRepo)
			result, err := svc.GetGasPolicy(context.Background(), tt.chainID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.chainID, result.ChainID)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetRpcEndpoints(t *testing.T) {
	tests := []struct {
		name        string
		chainID     domain.ChainID
		setupMock   func(*MockRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name:    "successful get RPC endpoints",
			chainID: "eip155:1",
			setupMock: func(mockRepo *MockRepository) {
				expected := &domain.ChainRpcEndpoints{
					ChainID: "eip155:1",
					Endpoints: []domain.RpcEndpoint{
						{
							URL:      "https://eth-mainnet.alchemyapi.io/v2/your-api-key",
							Priority: 1,
							Weight:   100,
							AuthType: domain.RpcAuthKey,
							Active:   true,
						},
					},
					RegistryVersion: "1.0.0",
				}
				mockRepo.On("GetRpcEndpoints", mock.Anything, "eip155:1").Return(expected, nil)
			},
			expectError: false,
		},
		{
			name:    "empty chain ID",
			chainID: "",
			setupMock: func(mockRepo *MockRepository) {
				// No mock setup needed as validation should fail first
			},
			expectError: true,
			errorMsg:    "chain_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			tt.setupMock(mockRepo)

			svc := service.New(mockRepo)
			result, err := svc.GetRpcEndpoints(context.Background(), tt.chainID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.chainID, result.ChainID)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_BumpVersion(t *testing.T) {
	tests := []struct {
		name        string
		chainID     domain.ChainID
		reason      string
		setupMock   func(*MockRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name:    "successful bump version",
			chainID: "eip155:1",
			reason:  "test reason",
			setupMock: func(mockRepo *MockRepository) {
				mockRepo.On("BumpVersion", mock.Anything, "eip155:1", "test reason").Return("1.0.1234567890", nil)
			},
			expectError: false,
		},
		{
			name:        "empty chain ID",
			chainID:     "",
			reason:      "test reason",
			setupMock:   func(mockRepo *MockRepository) {},
			expectError: true,
			errorMsg:    "chain_id is required",
		},
		{
			name:        "empty reason",
			chainID:     "eip155:1",
			reason:      "",
			setupMock:   func(mockRepo *MockRepository) {},
			expectError: true,
			errorMsg:    "reason is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			tt.setupMock(mockRepo)

			svc := service.New(mockRepo)
			ok, newVersion, err := svc.BumpVersion(context.Background(), tt.chainID, tt.reason)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.False(t, ok)
				assert.Empty(t, newVersion)
			} else {
				assert.NoError(t, err)
				assert.True(t, ok)
				assert.NotEmpty(t, newVersion)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetContractMeta(t *testing.T) {
	tests := []struct {
		name        string
		chainID     domain.ChainID
		address     domain.Address
		setupMock   func(*MockRepository)
		expectError bool
		errorMsg    string
	}{
		{
			name:    "successful get contract meta",
			chainID: "eip155:1",
			address: "0x1234567890123456789012345678901234567890",
			setupMock: func(mockRepo *MockRepository) {
				expected := &domain.ContractMeta{
					ChainID: "eip155:1",
					Contract: domain.Contract{
						Name:     "Test Contract",
						Address:  "0x1234567890123456789012345678901234567890",
						Standard: domain.StdERC721,
					},
					RegistryVersion: "1.0.0",
				}
				mockRepo.On("GetContractMeta", mock.Anything, "eip155:1", "0x1234567890123456789012345678901234567890").Return(expected, nil)
			},
			expectError: false,
		},
		{
			name:        "empty chain ID",
			chainID:     "",
			address:     "0x1234567890123456789012345678901234567890",
			setupMock:   func(mockRepo *MockRepository) {},
			expectError: true,
			errorMsg:    "chain_id is required",
		},
		{
			name:        "empty address",
			chainID:     "eip155:1",
			address:     "",
			setupMock:   func(mockRepo *MockRepository) {},
			expectError: true,
			errorMsg:    "address is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			tt.setupMock(mockRepo)

			svc := service.New(mockRepo)
			result, err := svc.GetContractMeta(context.Background(), tt.chainID, tt.address)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.chainID, result.ChainID)
				assert.Equal(t, tt.address, result.Contract.Address)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestService_GetAbiBlob(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := service.New(mockRepo)

	ctx := context.Background()
	sha := domain.Sha256("abc123")
	expectedABI := []byte(`{"abi": []}`)
	expectedETag := "etag-123"

	t.Run("success", func(t *testing.T) {
		mockRepo.On("GetAbiBlob", ctx, sha).Return(expectedABI, expectedETag, nil).Once()

		abiJSON, etag, err := svc.GetAbiBlob(ctx, sha)

		assert.NoError(t, err)
		assert.Equal(t, expectedABI, abiJSON)
		assert.Equal(t, expectedETag, etag)
		mockRepo.AssertExpectations(t)
	})

	t.Run("empty sha", func(t *testing.T) {
		abiJSON, etag, err := svc.GetAbiBlob(ctx, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "abi_sha256 is required")
		assert.Nil(t, abiJSON)
		assert.Empty(t, etag)
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo.On("GetAbiBlob", ctx, sha).Return(nil, "", fmt.Errorf("db error")).Once()

		abiJSON, etag, err := svc.GetAbiBlob(ctx, sha)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get ABI blob from repository")
		assert.Nil(t, abiJSON)
		assert.Empty(t, etag)
		mockRepo.AssertExpectations(t)
	})
}

func TestService_ResolveProxy(t *testing.T) {
	mockRepo := &MockRepository{}
	svc := service.New(mockRepo)

	ctx := context.Background()
	chainID := domain.ChainID("eip155:1")
	address := domain.Address("0x1234567890123456789012345678901234567890")
	expectedImplAddress := domain.Address("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")
	expectedAbiSha256 := domain.Sha256("def456")

	t.Run("success", func(t *testing.T) {
		mockRepo.On("ResolveProxy", ctx, chainID, address).Return(expectedImplAddress, expectedAbiSha256, nil).Once()

		implAddress, abiSha256, err := svc.ResolveProxy(ctx, chainID, address)

		assert.NoError(t, err)
		assert.Equal(t, expectedImplAddress, implAddress)
		assert.Equal(t, expectedAbiSha256, abiSha256)
		mockRepo.AssertExpectations(t)
	})

	t.Run("empty chain_id", func(t *testing.T) {
		implAddress, abiSha256, err := svc.ResolveProxy(ctx, "", address)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "chain_id is required")
		assert.Empty(t, implAddress)
		assert.Empty(t, abiSha256)
	})

	t.Run("empty address", func(t *testing.T) {
		implAddress, abiSha256, err := svc.ResolveProxy(ctx, chainID, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "address is required")
		assert.Empty(t, implAddress)
		assert.Empty(t, abiSha256)
	})

	t.Run("repository error", func(t *testing.T) {
		mockRepo.On("ResolveProxy", ctx, chainID, address).Return("", "", fmt.Errorf("db error")).Once()

		implAddress, abiSha256, err := svc.ResolveProxy(ctx, chainID, address)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve proxy from repository")
		assert.Empty(t, implAddress)
		assert.Empty(t, abiSha256)
		mockRepo.AssertExpectations(t)
	})
}
