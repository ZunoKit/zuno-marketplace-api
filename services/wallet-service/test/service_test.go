package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/service"
)

// MockWalletRepository is a mock implementation of WalletRepository
type MockWalletRepository struct {
	mock.Mock
}

func (m *MockWalletRepository) WithTx(ctx context.Context, fn func(domain.TxWalletRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

// MockTxWalletRepository is a mock implementation of TxWalletRepository
type MockTxWalletRepository struct {
	mock.Mock
}

func (m *MockTxWalletRepository) AcquireAccountLock(ctx context.Context, accountID string) error {
	args := m.Called(ctx, accountID)
	return args.Error(0)
}

func (m *MockTxWalletRepository) AcquireAddressLock(ctx context.Context, chainID, address string) error {
	args := m.Called(ctx, chainID, address)
	return args.Error(0)
}

func (m *MockTxWalletRepository) GetByAccountIDTx(ctx context.Context, accountID string) (*domain.WalletLink, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

func (m *MockTxWalletRepository) GetByAddressTx(ctx context.Context, chainID, address string) (*domain.WalletLink, error) {
	args := m.Called(ctx, chainID, address)
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

func (m *MockTxWalletRepository) InsertWalletTx(ctx context.Context, link domain.WalletLink) (*domain.WalletLink, error) {
	args := m.Called(ctx, link)
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

func (m *MockTxWalletRepository) UpdateWalletMetaTx(ctx context.Context, id domain.WalletID, isPrimary *bool, verifiedAt *time.Time, lastSeen *time.Time, label *string) (*domain.WalletLink, error) {
	args := m.Called(ctx, id, isPrimary, verifiedAt, lastSeen, label)
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

func (m *MockTxWalletRepository) GetPrimaryByUserTx(ctx context.Context, userID domain.UserID) (*domain.WalletLink, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

func (m *MockTxWalletRepository) GetPrimaryByUserChainTx(ctx context.Context, userID domain.UserID, chainID domain.ChainID) (*domain.WalletLink, error) {
	args := m.Called(ctx, userID, chainID)
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

func (m *MockTxWalletRepository) DemoteOtherPrimariesTx(ctx context.Context, userID domain.UserID, chainID domain.ChainID, keepID domain.WalletID) error {
	args := m.Called(ctx, userID, chainID, keepID)
	return args.Error(0)
}

func (m *MockTxWalletRepository) UpdateWalletAddressTx(ctx context.Context, id domain.WalletID, chainID domain.ChainID, address domain.Address) (*domain.WalletLink, error) {
	args := m.Called(ctx, id, chainID, address)
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

// WalletServiceTestSuite defines the test suite for WalletService
type WalletServiceTestSuite struct {
	suite.Suite
	walletService *service.Service
	mockRepo      *MockWalletRepository
}

func (suite *WalletServiceTestSuite) SetupTest() {
	suite.mockRepo = new(MockWalletRepository)
	suite.walletService = service.NewWalletService(suite.mockRepo).(*service.Service)
}

func (suite *WalletServiceTestSuite) TestUpsertLink_NewWallet() {
	ctx := context.Background()
	link := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainID:   "eip155:1",
		IsPrimary: true,
	}

	mockTxRepo := new(MockTxWalletRepository)

	// Expected result after insertion
	insertedLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     link.UserID,
		AccountID:  link.AccountID,
		Address:    link.Address,
		ChainID:    link.ChainID,
		IsPrimary:  true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		VerifiedAt: &time.Time{},
	}

	// Mock transaction
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxWalletRepository) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(domain.TxWalletRepository) error)

			// Mock transaction operations for new wallet scenario
			mockTxRepo.On("AcquireAccountLock", ctx, link.AccountID).Return(nil)
			mockTxRepo.On("AcquireAddressLock", ctx, link.ChainID, link.Address).Return(nil)
			mockTxRepo.On("GetByAddressTx", ctx, link.ChainID, link.Address).Return((*domain.WalletLink)(nil), domain.ErrWalletNotFound)
			mockTxRepo.On("GetByAccountIDTx", ctx, link.AccountID).Return((*domain.WalletLink)(nil), domain.ErrWalletNotFound)
			// GetPrimaryByUserChainTx is NOT called when IsPrimary is true - it goes directly to DemoteOtherPrimariesTx
			mockTxRepo.On("DemoteOtherPrimariesTx", ctx, link.UserID, link.ChainID, "").Return(nil)
			mockTxRepo.On("InsertWalletTx", ctx, mock.AnythingOfType("domain.WalletLink")).Return(insertedLink, nil)

			fn(mockTxRepo)
		}).Return(nil)

	result, err := suite.walletService.UpsertLink(ctx, link)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(insertedLink.ID, result.Link.ID)
	suite.True(result.Created)
	suite.True(result.PrimaryChanged)
	suite.mockRepo.AssertExpectations(suite.T())
	mockTxRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestUpsertLink_ExistingWallet() {
	ctx := context.Background()
	link := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainID:   "eip155:1",
		IsPrimary: false,
	}

	existingWallet := &domain.WalletLink{
		ID:        "wallet-789",
		UserID:    link.UserID,
		AccountID: link.AccountID,
		Address:   link.Address,
		ChainID:   link.ChainID,
		IsPrimary: false,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	mockTxRepo := new(MockTxWalletRepository)

	// Mock transaction
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxWalletRepository) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(domain.TxWalletRepository) error)

			// Mock transaction operations for existing wallet scenario
			mockTxRepo.On("AcquireAccountLock", ctx, link.AccountID).Return(nil)
			mockTxRepo.On("AcquireAddressLock", ctx, link.ChainID, link.Address).Return(nil)
			mockTxRepo.On("GetByAddressTx", ctx, link.ChainID, link.Address).Return(existingWallet, nil)
			mockTxRepo.On("GetByAccountIDTx", ctx, link.AccountID).Return(existingWallet, nil)
			mockTxRepo.On("UpdateWalletMetaTx", ctx, existingWallet.ID, (*bool)(nil), mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time"), (*string)(nil)).Return(existingWallet, nil)

			fn(mockTxRepo)
		}).Return(nil)

	result, err := suite.walletService.UpsertLink(ctx, link)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(existingWallet.ID, result.Link.ID)
	suite.False(result.Created)
	suite.False(result.PrimaryChanged)
	suite.mockRepo.AssertExpectations(suite.T())
	mockTxRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestUpsertLink_PromoteToPrimary() {
	ctx := context.Background()
	link := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainID:   "eip155:1",
		IsPrimary: true, // Request to promote to primary
	}

	existingWallet := &domain.WalletLink{
		ID:        "wallet-789",
		UserID:    link.UserID,
		AccountID: link.AccountID,
		Address:   link.Address,
		ChainID:   link.ChainID,
		IsPrimary: false, // Currently not primary
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	updatedWallet := &domain.WalletLink{
		ID:        existingWallet.ID,
		UserID:    existingWallet.UserID,
		AccountID: existingWallet.AccountID,
		Address:   existingWallet.Address,
		ChainID:   existingWallet.ChainID,
		IsPrimary: true, // Now primary
		CreatedAt: existingWallet.CreatedAt,
		UpdatedAt: time.Now(),
	}

	mockTxRepo := new(MockTxWalletRepository)

	// Mock transaction
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxWalletRepository) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(domain.TxWalletRepository) error)

			// Mock transaction operations for promote to primary scenario
			mockTxRepo.On("AcquireAccountLock", ctx, link.AccountID).Return(nil)
			mockTxRepo.On("AcquireAddressLock", ctx, link.ChainID, link.Address).Return(nil)
			mockTxRepo.On("GetByAddressTx", ctx, link.ChainID, link.Address).Return(existingWallet, nil)
			mockTxRepo.On("GetByAccountIDTx", ctx, link.AccountID).Return(existingWallet, nil)
			mockTxRepo.On("DemoteOtherPrimariesTx", ctx, existingWallet.UserID, existingWallet.ChainID, existingWallet.ID).Return(nil)

			isPrimary := true
			mockTxRepo.On("UpdateWalletMetaTx", ctx, existingWallet.ID, &isPrimary, mock.AnythingOfType("*time.Time"), mock.AnythingOfType("*time.Time"), (*string)(nil)).Return(updatedWallet, nil)

			fn(mockTxRepo)
		}).Return(nil)

	result, err := suite.walletService.UpsertLink(ctx, link)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(updatedWallet.ID, result.Link.ID)
	suite.True(result.Link.IsPrimary)
	suite.False(result.Created)
	suite.True(result.PrimaryChanged)
	suite.mockRepo.AssertExpectations(suite.T())
	mockTxRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestUpsertLink_UnauthorizedAccess() {
	ctx := context.Background()
	link := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainID:   "eip155:1",
		IsPrimary: false,
	}

	// Existing wallet belongs to different user
	// Create two different wallet records to trigger unauthorized access check
	existingWalletByAddress := &domain.WalletLink{
		ID:        "wallet-789",
		UserID:    "different-user-999", // Different user!
		AccountID: "other-account",
		Address:   link.Address,
		ChainID:   link.ChainID,
		IsPrimary: false,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	existingWalletByAccount := &domain.WalletLink{
		ID:        "wallet-456",
		UserID:    "another-user-888", // Also different user!
		AccountID: link.AccountID,
		Address:   "0xDifferentAddress000000000000000000000000",
		ChainID:   link.ChainID,
		IsPrimary: false,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	mockTxRepo := new(MockTxWalletRepository)

	// Mock transaction
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxWalletRepository) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(domain.TxWalletRepository) error)

			// Mock transaction operations that will detect unauthorized access
			// Different wallet IDs will trigger the unauthorized access check
			mockTxRepo.On("AcquireAccountLock", ctx, link.AccountID).Return(nil)
			mockTxRepo.On("AcquireAddressLock", ctx, link.ChainID, link.Address).Return(nil)
			mockTxRepo.On("GetByAddressTx", ctx, link.ChainID, link.Address).Return(existingWalletByAddress, nil)
			mockTxRepo.On("GetByAccountIDTx", ctx, link.AccountID).Return(existingWalletByAccount, nil)

			fn(mockTxRepo)
		}).Return(domain.ErrUnauthorizedAccess)

	result, err := suite.walletService.UpsertLink(ctx, link)

	suite.Error(err)
	suite.Nil(result)
	suite.Equal(domain.ErrUnauthorizedAccess, err)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestUpsertLink_InvalidInput() {
	ctx := context.Background()

	testCases := []struct {
		name string
		link domain.WalletLink
	}{
		{
			name: "empty_user_id",
			link: domain.WalletLink{
				UserID:    "",
				AccountID: "account-456",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainID:   "eip155:1",
			},
		},
		{
			name: "empty_account_id",
			link: domain.WalletLink{
				UserID:    "user-123",
				AccountID: "",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainID:   "eip155:1",
			},
		},
		{
			name: "invalid_address",
			link: domain.WalletLink{
				UserID:    "user-123",
				AccountID: "account-456",
				Address:   "invalid-address",
				ChainID:   "eip155:1",
			},
		},
		{
			name: "invalid_chain_id",
			link: domain.WalletLink{
				UserID:    "user-123",
				AccountID: "account-456",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainID:   "invalid-chain",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.walletService.UpsertLink(ctx, tc.link)

			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}

func (suite *WalletServiceTestSuite) TestUpsertLink_AddressUpdate() {
	ctx := context.Background()
	link := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x9876543210987654321098765432109876543210", // New address
		ChainID:   "eip155:1",
		IsPrimary: false,
	}

	// Existing wallet with same account but different address
	existingWallet := &domain.WalletLink{
		ID:        "wallet-789",
		UserID:    link.UserID,
		AccountID: link.AccountID,
		Address:   "0x1234567890123456789012345678901234567890", // Old address
		ChainID:   link.ChainID,
		IsPrimary: false,
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	updatedWallet := &domain.WalletLink{
		ID:        existingWallet.ID,
		UserID:    existingWallet.UserID,
		AccountID: existingWallet.AccountID,
		Address:   link.Address, // Updated address
		ChainID:   existingWallet.ChainID,
		IsPrimary: existingWallet.IsPrimary,
		CreatedAt: existingWallet.CreatedAt,
		UpdatedAt: time.Now(),
	}

	mockTxRepo := new(MockTxWalletRepository)

	// Mock transaction
	suite.mockRepo.On("WithTx", ctx, mock.AnythingOfType("func(domain.TxWalletRepository) error")).
		Run(func(args mock.Arguments) {
			fn := args.Get(1).(func(domain.TxWalletRepository) error)

			// Mock transaction operations for address update scenario
			mockTxRepo.On("AcquireAccountLock", ctx, link.AccountID).Return(nil)
			mockTxRepo.On("AcquireAddressLock", ctx, link.ChainID, link.Address).Return(nil)
			mockTxRepo.On("GetByAddressTx", ctx, link.ChainID, link.Address).Return((*domain.WalletLink)(nil), domain.ErrWalletNotFound)
			mockTxRepo.On("GetByAccountIDTx", ctx, link.AccountID).Return(existingWallet, nil)
			mockTxRepo.On("GetByAddressTx", ctx, link.ChainID, link.Address).Return((*domain.WalletLink)(nil), domain.ErrWalletNotFound)
			mockTxRepo.On("UpdateWalletAddressTx", ctx, existingWallet.ID, link.ChainID, link.Address).Return(updatedWallet, nil)

			fn(mockTxRepo)
		}).Return(nil)

	result, err := suite.walletService.UpsertLink(ctx, link)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(updatedWallet.ID, result.Link.ID)
	suite.Equal(link.Address, result.Link.Address)
	suite.False(result.Created)
	suite.False(result.PrimaryChanged)
	suite.mockRepo.AssertExpectations(suite.T())
	mockTxRepo.AssertExpectations(suite.T())
}

func TestWalletServiceTestSuite(t *testing.T) {
	suite.Run(t, new(WalletServiceTestSuite))
}

// Additional unit tests for validation functions
func TestValidateWalletLink(t *testing.T) {
	service := &service.Service{}

	validLink := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainID:   "eip155:1",
		IsPrimary: false,
	}

	assert.NoError(t, service.ValidateWalletLink(validLink))

	// Test invalid cases
	invalidLinks := []domain.WalletLink{
		{
			UserID:    "",
			AccountID: "account-456",
			Address:   "0x1234567890123456789012345678901234567890",
			ChainID:   "eip155:1",
		},
		{
			UserID:    "user-123",
			AccountID: "",
			Address:   "0x1234567890123456789012345678901234567890",
			ChainID:   "eip155:1",
		},
		{
			UserID:    "user-123",
			AccountID: "account-456",
			Address:   "",
			ChainID:   "eip155:1",
		},
		{
			UserID:    "user-123",
			AccountID: "account-456",
			Address:   "0x1234567890123456789012345678901234567890",
			ChainID:   "",
		},
	}

	for i, link := range invalidLinks {
		assert.Error(t, service.ValidateWalletLink(link), "Expected link %d to be invalid", i)
	}
}

func TestNormalizeAddress(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"0xAbCdEf1234567890123456789012345678901234", "0xabcdef1234567890123456789012345678901234"},
		{"0x1234567890123456789012345678901234567890", "0x1234567890123456789012345678901234567890"},
		{"0XABCDEF1234567890123456789012345678901234", "0xabcdef1234567890123456789012345678901234"},
	}

	for _, tc := range testCases {
		result := service.NormalizeAddress(tc.input)
		assert.Equal(t, tc.expected, result, "Address normalization failed for %s", tc.input)
	}
}

func TestNormalizeChainID(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"EIP155:1", "eip155:1"},
		{"eip155:1", "eip155:1"},
		{"EIP155:137", "eip155:137"},
		{"COSMOS:COSMOSHUB-4", "cosmos:cosmoshub-4"},
	}

	for _, tc := range testCases {
		result := service.NormalizeChainID(tc.input)
		assert.Equal(t, tc.expected, result, "ChainID normalization failed for %s", tc.input)
	}
}

func TestIsValidEthereumAddress(t *testing.T) {
	validAddresses := []string{
		"0x1234567890123456789012345678901234567890",
		"0xAbCdEf1234567890123456789012345678901234",
		"0x0000000000000000000000000000000000000000",
		"0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF",
	}

	invalidAddresses := []string{
		"",
		"1234567890123456789012345678901234567890",
		"0x123",
		"0x12345678901234567890123456789012345678901",
		"0x123456789012345678901234567890123456789g",
		"0X1234567890123456789012345678901234567890",
	}

	for _, address := range validAddresses {
		assert.True(t, service.IsValidEthereumAddress(address), "Expected %s to be valid", address)
	}

	for _, address := range invalidAddresses {
		assert.False(t, service.IsValidEthereumAddress(address), "Expected %s to be invalid", address)
	}
}

func TestIsValidChainID(t *testing.T) {
	validChainIDs := []string{
		"eip155:1",
		"eip155:137",
		"cosmos:cosmoshub-4",
		"bip122:000000000019d6689c085ae165831e93",
	}

	invalidChainIDs := []string{
		"",
		"eip155",
		":1",
		"eip155:",
		"invalid-format",
	}

	for _, chainID := range validChainIDs {
		assert.True(t, service.IsValidChainID(chainID), "Expected %s to be valid", chainID)
	}

	for _, chainID := range invalidChainIDs {
		assert.False(t, service.IsValidChainID(chainID), "Expected %s to be invalid", chainID)
	}
}

// Benchmark tests
func BenchmarkUpsertLink_ExistingWallet(b *testing.B) {
	mockRepo := new(MockWalletRepository)
	walletService := service.NewWalletService(mockRepo)
	ctx := context.Background()

	link := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainID:   "eip155:1",
		IsPrimary: false,
	}

	existingWallet := &domain.WalletLink{
		ID:        "wallet-789",
		UserID:    link.UserID,
		AccountID: link.AccountID,
		Address:   link.Address,
		ChainID:   link.ChainID,
		IsPrimary: false,
	}
	_ = existingWallet

	mockRepo.On("WithTx", mock.Anything, mock.Anything).Return(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		walletService.UpsertLink(ctx, link)
	}
}

func BenchmarkValidateWalletLink(b *testing.B) {
	service := &service.Service{}
	link := domain.WalletLink{
		UserID:    "user-123",
		AccountID: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainID:   "eip155:1",
		IsPrimary: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.ValidateWalletLink(link)
	}
}

func BenchmarkNormalizeAddress(b *testing.B) {
	address := "0xAbCdEf1234567890123456789012345678901234"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.NormalizeAddress(address)
	}
}
