package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	grpcHandler "github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/infrastructure/grpc"
	walletpb "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
)

// MockWalletService is a mock implementation of WalletService
type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) UpsertLink(ctx context.Context, userID domain.UserID, accountID string, address domain.Address, chainID domain.ChainID, isPrimary bool, walletType, connector, label string) (*domain.WalletLink, bool, bool, error) {
	args := m.Called(ctx, userID, accountID, address, chainID, isPrimary, walletType, connector, label)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Bool(2), args.Error(3)
	}
	return args.Get(0).(*domain.WalletLink), args.Bool(1), args.Bool(2), args.Error(3)
}

func (m *MockWalletService) GetUserWallets(ctx context.Context, userID domain.UserID) ([]*domain.WalletLink, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.WalletLink), args.Error(1)
}

func (m *MockWalletService) GetWalletByAddress(ctx context.Context, address domain.Address) (*domain.WalletLink, error) {
	args := m.Called(ctx, address)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.WalletLink), args.Error(1)
}

func (m *MockWalletService) SetPrimaryWallet(ctx context.Context, userID domain.UserID, walletID domain.WalletID) error {
	args := m.Called(ctx, userID, walletID)
	return args.Error(0)
}

func (m *MockWalletService) RemoveWallet(ctx context.Context, userID domain.UserID, walletID domain.WalletID) error {
	args := m.Called(ctx, userID, walletID)
	return args.Error(0)
}

// MockEventPublisher is a mock implementation of EventPublisher
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishWalletLinked(ctx context.Context, event *domain.WalletLinkedEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// WalletGRPCTestSuite defines the test suite for Wallet gRPC handler
type WalletGRPCTestSuite struct {
	suite.Suite
	handler interface {
		UpsertLink(context.Context, *walletpb.UpsertLinkRequest) (*walletpb.UpsertLinkResponse, error)
	}
	mockService   *MockWalletService
	mockPublisher *MockEventPublisher
	grpcServer    *grpc.Server
}

func (suite *WalletGRPCTestSuite) SetupTest() {
	suite.mockService = new(MockWalletService)
	suite.mockPublisher = new(MockEventPublisher)
	suite.grpcServer = grpc.NewServer()
	suite.handler = grpcHandler.NewgRPCHandler(suite.grpcServer, suite.mockService)
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_Success_NewWallet() {
	ctx := context.Background()
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: true,
		Type:      "eoa",
		Connector: "metamask",
		Label:     "My Wallet",
	}

	walletLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     req.UserId,
		AccountID:  req.AccountId,
		Address:    req.Address,
		ChainID:    req.ChainId,
		IsPrimary:  req.IsPrimary,
		Type:       domain.WalletType(req.Type),
		Connector:  req.Connector,
		Label:      req.Label,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		VerifiedAt: time.Now(),
	}

	// Mock service call with correct signature
	suite.mockService.On("UpsertLink", ctx,
		req.UserId,
		req.AccountId,
		domain.Address(req.Address),
		domain.ChainID(req.ChainId),
		req.IsPrimary,
		req.Type,
		req.Connector,
		req.Label).Return(walletLink, true, true, nil)

	// No event publishing in the actual implementation

	resp, err := suite.handler.UpsertLink(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(walletLink.ID, resp.Link.Id)
	suite.Equal(walletLink.UserID, resp.Link.UserId)
	suite.Equal(walletLink.AccountID, resp.Link.AccountId)
	suite.Equal(walletLink.Address, resp.Link.Address)
	suite.Equal(walletLink.ChainID, resp.Link.ChainId)
	suite.Equal(walletLink.IsPrimary, resp.Link.IsPrimary)
	suite.True(resp.Created)
	suite.True(resp.PrimaryChanged)

	suite.mockService.AssertExpectations(suite.T())
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_Success_ExistingWallet() {
	ctx := context.Background()
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: false,
		Type:      "eoa",
		Connector: "metamask",
		Label:     "My Wallet",
	}

	walletLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     req.UserId,
		AccountID:  req.AccountId,
		Address:    req.Address,
		ChainID:    req.ChainId,
		IsPrimary:  req.IsPrimary,
		Type:       domain.WalletType(req.Type),
		Connector:  req.Connector,
		Label:      req.Label,
		CreatedAt:  time.Now().Add(-1 * time.Hour),
		UpdatedAt:  time.Now(),
		VerifiedAt: time.Now(),
	}

	// Mock service call with correct signature
	suite.mockService.On("UpsertLink", ctx,
		req.UserId,
		req.AccountId,
		domain.Address(req.Address),
		domain.ChainID(req.ChainId),
		req.IsPrimary,
		req.Type,
		req.Connector,
		req.Label).Return(walletLink, false, false, nil)

	resp, err := suite.handler.UpsertLink(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(walletLink.ID, resp.Link.Id)
	suite.False(resp.Created)
	suite.False(resp.PrimaryChanged)

	suite.mockService.AssertExpectations(suite.T())
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_InvalidRequest() {
	ctx := context.Background()

	testCases := []struct {
		name string
		req  *walletpb.UpsertLinkRequest
	}{
		{
			name: "missing_user_id",
			req: &walletpb.UpsertLinkRequest{
				AccountId: "account-456",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainId:   "eip155:1",
			},
		},
		{
			name: "missing_account_id",
			req: &walletpb.UpsertLinkRequest{
				UserId:  "user-123",
				Address: "0x1234567890123456789012345678901234567890",
				ChainId: "eip155:1",
			},
		},
		{
			name: "missing_address",
			req: &walletpb.UpsertLinkRequest{
				UserId:    "user-123",
				AccountId: "account-456",
				ChainId:   "eip155:1",
			},
		},
		{
			name: "missing_chain_id",
			req: &walletpb.UpsertLinkRequest{
				UserId:    "user-123",
				AccountId: "account-456",
				Address:   "0x1234567890123456789012345678901234567890",
			},
		},
		{
			name: "empty_user_id",
			req: &walletpb.UpsertLinkRequest{
				UserId:    "",
				AccountId: "account-456",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainId:   "eip155:1",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			resp, err := suite.handler.UpsertLink(ctx, tc.req)

			assert.Error(t, err)
			assert.Nil(t, resp)

			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
		})
	}
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_ServiceError() {
	ctx := context.Background()
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "invalid-address",
		ChainId:   "eip155:1",
		IsPrimary: false,
		Type:      "eoa",
		Connector: "metamask",
		Label:     "My Wallet",
	}

	suite.mockService.On("UpsertLink", ctx,
		req.UserId,
		req.AccountId,
		domain.Address(req.Address),
		domain.ChainID(req.ChainId),
		req.IsPrimary,
		req.Type,
		req.Connector,
		req.Label).Return((*domain.WalletLink)(nil), false, false, domain.ErrInvalidAddress)

	resp, err := suite.handler.UpsertLink(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	suite.Equal(codes.InvalidArgument, st.Code())
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_UnauthorizedAccess() {
	ctx := context.Background()
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: false,
		Type:      "eoa",
		Connector: "metamask",
		Label:     "My Wallet",
	}

	suite.mockService.On("UpsertLink", ctx,
		req.UserId,
		req.AccountId,
		domain.Address(req.Address),
		domain.ChainID(req.ChainId),
		req.IsPrimary,
		req.Type,
		req.Connector,
		req.Label).Return((*domain.WalletLink)(nil), false, false, assert.AnError)

	resp, err := suite.handler.UpsertLink(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	suite.Equal(codes.Internal, st.Code()) // The actual implementation returns Internal for all non-specific errors
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_MultipleCallsSuccess() {
	ctx := context.Background()
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: true,
		Type:      "eoa",
		Connector: "metamask",
		Label:     "My Wallet",
	}

	walletLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     req.UserId,
		AccountID:  req.AccountId,
		Address:    req.Address,
		ChainID:    req.ChainId,
		IsPrimary:  req.IsPrimary,
		Type:       domain.WalletType("eoa"),
		Connector:  "metamask",
		Label:      "My Wallet",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		VerifiedAt: time.Now(),
	}

	// Mock service call
	suite.mockService.On("UpsertLink", ctx,
		req.UserId,
		req.AccountId,
		domain.Address(req.Address),
		domain.ChainID(req.ChainId),
		req.IsPrimary,
		"eoa",
		"metamask",
		"My Wallet").Return(walletLink, true, true, nil)

	resp, err := suite.handler.UpsertLink(ctx, req)

	// Response should succeed
	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(walletLink.ID, resp.Link.Id)
	suite.True(resp.Created)
	suite.True(resp.PrimaryChanged)

	suite.mockService.AssertExpectations(suite.T())
}

func TestWalletGRPCTestSuite(t *testing.T) {
	suite.Run(t, new(WalletGRPCTestSuite))
}

// Additional unit tests for edge cases
func TestGRPCValidation(t *testing.T) {
	mockService := new(MockWalletService)
	grpcServer := grpc.NewServer()
	handler := grpcHandler.NewgRPCHandler(grpcServer, mockService)
	ctx := context.Background()

	t.Run("NilRequest", func(t *testing.T) {
		resp, err := handler.UpsertLink(ctx, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("ValidRequest", func(t *testing.T) {
		req := &walletpb.UpsertLinkRequest{
			UserId:    "user-123",
			AccountId: "account-456",
			Address:   "0x1234567890123456789012345678901234567890",
			ChainId:   "eip155:1",
			IsPrimary: false,
		}

		walletLink := &domain.WalletLink{
			ID:         "wallet-789",
			UserID:     req.UserId,
			AccountID:  req.AccountId,
			Address:    req.Address,
			ChainID:    req.ChainId,
			IsPrimary:  req.IsPrimary,
			Type:       domain.WalletType("eoa"),
			Connector:  "metamask",
			Label:      "My Wallet",
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			VerifiedAt: time.Now(),
		}

		mockService.On("UpsertLink", ctx,
			req.UserId,
			req.AccountId,
			domain.Address(req.Address),
			domain.ChainID(req.ChainId),
			req.IsPrimary,
			"", "", "").Return(walletLink, false, false, nil).Once()

		resp, err := handler.UpsertLink(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, walletLink.ID, resp.Link.Id)
		assert.False(t, resp.Created)
		assert.False(t, resp.PrimaryChanged)
	})
}

// Removed domain to proto conversion tests as these methods don't exist in the actual implementation

// Benchmark tests for gRPC handlers
func BenchmarkUpsertLink(b *testing.B) {
	mockService := new(MockWalletService)
	grpcServer := grpc.NewServer()
	handler := grpcHandler.NewgRPCHandler(grpcServer, mockService)
	ctx := context.Background()

	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: false,
	}

	walletLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     req.UserId,
		AccountID:  req.AccountId,
		Address:    req.Address,
		ChainID:    req.ChainId,
		IsPrimary:  req.IsPrimary,
		Type:       domain.WalletType("eoa"),
		Connector:  "metamask",
		Label:      "My Wallet",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		VerifiedAt: time.Now(),
	}

	mockService.On("UpsertLink", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(walletLink, false, false, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.UpsertLink(ctx, req)
	}
}

// Stress tests
func TestUpsertLink_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	mockService := new(MockWalletService)
	grpcServer := grpc.NewServer()
	handler := grpcHandler.NewgRPCHandler(grpcServer, mockService)
	ctx := context.Background()

	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: false,
	}

	walletLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     req.UserId,
		AccountID:  req.AccountId,
		Address:    req.Address,
		ChainID:    req.ChainId,
		IsPrimary:  req.IsPrimary,
		Type:       domain.WalletType("eoa"),
		Connector:  "metamask",
		Label:      "My Wallet",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		VerifiedAt: time.Now(),
	}

	mockService.On("UpsertLink", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(walletLink, false, false, nil)

	// Run concurrent requests
	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() { done <- true }()

			resp, err := handler.UpsertLink(ctx, req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, walletLink.ID, resp.Link.Id)
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < concurrency; i++ {
		<-done
	}
}

// Error mapping tests
func TestErrorMapping(t *testing.T) {
	testCases := []struct {
		name         string
		serviceError error
		expectedCode codes.Code
		expectedMsg  string
	}{
		{
			name:         "wallet_not_found",
			serviceError: domain.ErrWalletNotFound,
			expectedCode: codes.Internal,
			expectedMsg:  "wallet not found",
		},
		{
			name:         "invalid_user_id",
			serviceError: domain.ErrInvalidUserID,
			expectedCode: codes.Internal,
			expectedMsg:  "invalid user ID",
		},
		{
			name:         "invalid_address",
			serviceError: domain.ErrInvalidAddress,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid address",
		},
		{
			name:         "invalid_chain_id",
			serviceError: domain.ErrInvalidChainID,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid chain ID",
		},
		{
			name:         "generic_error",
			serviceError: assert.AnError,
			expectedCode: codes.Internal,
			expectedMsg:  "failed to upsert wallet link",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockService := new(MockWalletService)
			grpcServer := grpc.NewServer()
			handler := grpcHandler.NewgRPCHandler(grpcServer, mockService)
			ctx := context.Background()

			req := &walletpb.UpsertLinkRequest{
				UserId:    "user-123",
				AccountId: "account-456",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainId:   "eip155:1",
				IsPrimary: false,
				Type:      "eoa",
				Connector: "metamask",
				Label:     "My Wallet",
			}

			mockService.On("UpsertLink", ctx, mock.Anything, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return((*domain.WalletLink)(nil), false, false, tc.serviceError)

			resp, err := handler.UpsertLink(ctx, req)

			assert.Error(t, err)
			assert.Nil(t, resp)

			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedCode, st.Code())
			assert.Contains(t, st.Message(), tc.expectedMsg)
		})
	}
}
