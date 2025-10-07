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

// MockWalletRepository mocks the wallet repository
type MockWalletRepository struct {
	mock.Mock
}

func (m *MockWalletRepository) CreateLink(ctx context.Context, link *domain.WalletLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *MockWalletRepository) GetLink(ctx context.Context, walletID domain.WalletID) (*domain.WalletLink, error) {
	args := m.Called(ctx, walletID)
	if link := args.Get(0); link != nil {
		return link.(*domain.WalletLink), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWalletRepository) GetLinkByUserAndAddress(ctx context.Context, userID domain.UserID, address domain.Address) (*domain.WalletLink, error) {
	args := m.Called(ctx, userID, address)
	if link := args.Get(0); link != nil {
		return link.(*domain.WalletLink), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWalletRepository) GetUserWallets(ctx context.Context, userID domain.UserID) ([]*domain.WalletLink, error) {
	args := m.Called(ctx, userID)
	if wallets := args.Get(0); wallets != nil {
		return wallets.([]*domain.WalletLink), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWalletRepository) GetWalletByAddress(ctx context.Context, address domain.Address) (*domain.WalletLink, error) {
	args := m.Called(ctx, address)
	if wallet := args.Get(0); wallet != nil {
		return wallet.(*domain.WalletLink), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockWalletRepository) UpdateLink(ctx context.Context, link *domain.WalletLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *MockWalletRepository) UpdatePrimaryWallet(ctx context.Context, userID domain.UserID, walletID domain.WalletID) error {
	args := m.Called(ctx, userID, walletID)
	return args.Error(0)
}

func (m *MockWalletRepository) DeleteLink(ctx context.Context, walletID domain.WalletID) error {
	args := m.Called(ctx, walletID)
	return args.Error(0)
}

// MockWalletEventPublisher mocks the event publisher
type MockWalletEventPublisher struct {
	mock.Mock
}

func (m *MockWalletEventPublisher) PublishWalletLinked(ctx context.Context, event *domain.WalletLinkedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// WalletServiceTestSuite defines the test suite
type WalletServiceTestSuite struct {
	suite.Suite
	walletService domain.WalletService
	mockRepo      *MockWalletRepository
	mockPublisher *MockWalletEventPublisher
}

func (suite *WalletServiceTestSuite) SetupTest() {
	suite.mockRepo = new(MockWalletRepository)
	suite.mockPublisher = new(MockWalletEventPublisher)
	suite.walletService = service.NewWalletService(suite.mockRepo, suite.mockPublisher)
}

func (suite *WalletServiceTestSuite) TestUpsertLink_CreateNew() {
	ctx := context.Background()
	userID := domain.UserID("user123")
	accountID := "0x1234567890123456789012345678901234567890"
	address := domain.Address("0x1234567890123456789012345678901234567890")
	chainID := domain.ChainID("eip155:1")

	// Mock no existing link
	suite.mockRepo.On("GetLinkByUserAndAddress", ctx, userID, address).Return(nil, domain.ErrWalletNotFound)

	// Mock no existing wallets (this will be primary)
	suite.mockRepo.On("GetUserWallets", ctx, userID).Return([]*domain.WalletLink{}, nil)

	// Mock create link
	suite.mockRepo.On("CreateLink", ctx, mock.AnythingOfType("*domain.WalletLink")).Return(nil)

	// Mock event publisher
	suite.mockPublisher.On("PublishWalletLinked", ctx, mock.AnythingOfType("*domain.WalletLinkedEvent")).Return(nil)

	// Act
	link, created, primaryChanged, err := suite.walletService.UpsertLink(
		ctx, userID, accountID, address, chainID, true, "eoa", "metamask", "My Wallet",
	)

	// Assert
	suite.NoError(err)
	suite.NotNil(link)
	suite.True(created)
	suite.False(primaryChanged)
	suite.True(link.IsPrimary) // First wallet is always primary
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestUpsertLink_UpdateExisting() {
	ctx := context.Background()
	userID := domain.UserID("user123")
	accountID := "0x1234567890123456789012345678901234567890"
	address := domain.Address("0x1234567890123456789012345678901234567890")
	chainID := domain.ChainID("eip155:1")

	existingLink := &domain.WalletLink{
		ID:        "wallet123",
		UserID:    userID,
		Address:   address,
		ChainID:   chainID,
		IsPrimary: false,
		CreatedAt: time.Now(),
	}

	// Mock existing link
	suite.mockRepo.On("GetLinkByUserAndAddress", ctx, userID, address).Return(existingLink, nil)

	// Mock update to primary
	suite.mockRepo.On("UpdatePrimaryWallet", ctx, userID, existingLink.ID).Return(nil)

	// Mock update link
	suite.mockRepo.On("UpdateLink", ctx, mock.AnythingOfType("*domain.WalletLink")).Return(nil)

	// Act
	link, created, primaryChanged, err := suite.walletService.UpsertLink(
		ctx, userID, accountID, address, chainID, true, "eoa", "walletconnect", "Updated Wallet",
	)

	// Assert
	suite.NoError(err)
	suite.NotNil(link)
	suite.False(created)
	suite.True(primaryChanged)
	suite.True(link.IsPrimary)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestGetUserWallets() {
	ctx := context.Background()
	userID := domain.UserID("user123")

	wallets := []*domain.WalletLink{
		{
			ID:        "wallet1",
			UserID:    userID,
			Address:   "0x1111111111111111111111111111111111111111",
			IsPrimary: true,
		},
		{
			ID:        "wallet2",
			UserID:    userID,
			Address:   "0x2222222222222222222222222222222222222222",
			IsPrimary: false,
		},
	}

	suite.mockRepo.On("GetUserWallets", ctx, userID).Return(wallets, nil)

	// Act
	result, err := suite.walletService.GetUserWallets(ctx, userID)

	// Assert
	suite.NoError(err)
	suite.Len(result, 2)
	suite.True(result[0].IsPrimary)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestSetPrimaryWallet() {
	ctx := context.Background()
	userID := domain.UserID("user123")
	walletID := domain.WalletID("wallet123")

	wallet := &domain.WalletLink{
		ID:     walletID,
		UserID: userID,
	}

	suite.mockRepo.On("GetLink", ctx, walletID).Return(wallet, nil)
	suite.mockRepo.On("UpdatePrimaryWallet", ctx, userID, walletID).Return(nil)

	// Act
	err := suite.walletService.SetPrimaryWallet(ctx, userID, walletID)

	// Assert
	suite.NoError(err)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestRemoveWallet_NotPrimary() {
	ctx := context.Background()
	userID := domain.UserID("user123")
	walletID := domain.WalletID("wallet123")

	wallet := &domain.WalletLink{
		ID:        walletID,
		UserID:    userID,
		IsPrimary: false,
	}

	suite.mockRepo.On("GetLink", ctx, walletID).Return(wallet, nil)
	suite.mockRepo.On("DeleteLink", ctx, walletID).Return(nil)

	// Act
	err := suite.walletService.RemoveWallet(ctx, userID, walletID)

	// Assert
	suite.NoError(err)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *WalletServiceTestSuite) TestRemoveWallet_PrimaryWithOthers() {
	ctx := context.Background()
	userID := domain.UserID("user123")
	walletID := domain.WalletID("wallet123")

	wallet := &domain.WalletLink{
		ID:        walletID,
		UserID:    userID,
		IsPrimary: true,
	}

	otherWallets := []*domain.WalletLink{
		wallet,
		{
			ID:        "wallet456",
			UserID:    userID,
			IsPrimary: false,
		},
	}

	suite.mockRepo.On("GetLink", ctx, walletID).Return(wallet, nil)
	suite.mockRepo.On("GetUserWallets", ctx, userID).Return(otherWallets, nil)
	suite.mockRepo.On("UpdatePrimaryWallet", ctx, userID, domain.WalletID("wallet456")).Return(nil)
	suite.mockRepo.On("DeleteLink", ctx, walletID).Return(nil)

	// Act
	err := suite.walletService.RemoveWallet(ctx, userID, walletID)

	// Assert
	suite.NoError(err)
	suite.mockRepo.AssertExpectations(suite.T())
}

func TestWalletServiceTestSuite(t *testing.T) {
	suite.Run(t, new(WalletServiceTestSuite))
}

// Additional unit tests
func TestWalletService_ValidationErrors(t *testing.T) {
	mockRepo := new(MockWalletRepository)
	mockPublisher := new(MockWalletEventPublisher)
	walletService := service.NewWalletService(mockRepo, mockPublisher)

	ctx := context.Background()

	t.Run("InvalidUserID", func(t *testing.T) {
		_, _, _, err := walletService.UpsertLink(
			ctx, "", "account", "0x123", "eip155:1", false, "", "", "",
		)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrInvalidUserID, err)
	})

	t.Run("InvalidAddress", func(t *testing.T) {
		_, _, _, err := walletService.UpsertLink(
			ctx, "user123", "account", "invalid", "eip155:1", false, "", "", "",
		)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrInvalidAddress, err)
	})

	t.Run("InvalidChainID", func(t *testing.T) {
		_, _, _, err := walletService.UpsertLink(
			ctx, "user123", "account", "0x1234567890123456789012345678901234567890", "invalid", false, "", "", "",
		)
		assert.Error(t, err)
		assert.Equal(t, domain.ErrInvalidChainID, err)
	})
}
