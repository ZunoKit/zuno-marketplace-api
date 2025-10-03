package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
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

func (m *MockWalletService) UpsertLink(ctx context.Context, link domain.WalletLink) (*domain.WalletUpsertResult, error) {
	args := m.Called(ctx, link)
	return args.Get(0).(*domain.WalletUpsertResult), args.Error(1)
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
	handler       *grpcHandler.WalletGRPCServer
	mockService   *MockWalletService
	mockPublisher *MockEventPublisher
}

func (suite *WalletGRPCTestSuite) SetupTest() {
	suite.mockService = new(MockWalletService)
	suite.mockPublisher = new(MockEventPublisher)
	suite.handler = grpcHandler.NewWalletGRPCServer(suite.mockService, suite.mockPublisher)
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_Success_NewWallet() {
	ctx := context.Background()
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: true,
	}

	walletLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     req.UserId,
		AccountID:  req.AccountId,
		Address:    req.Address,
		ChainID:    req.ChainId,
		IsPrimary:  req.IsPrimary,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		VerifiedAt: &time.Time{},
	}

	expectedResult := &domain.WalletUpsertResult{
		Link:           walletLink,
		Created:        true,
		PrimaryChanged: true,
	}

	// Mock service call
	suite.mockService.On("UpsertLink", ctx, mock.MatchedBy(func(link domain.WalletLink) bool {
		return link.UserID == req.UserId &&
			link.AccountID == req.AccountId &&
			link.Address == req.Address &&
			link.ChainID == req.ChainId &&
			link.IsPrimary == req.IsPrimary
	})).Return(expectedResult, nil)

	// Mock event publishing (should be called asynchronously)
	suite.mockPublisher.On("PublishWalletLinked", mock.Anything, mock.MatchedBy(func(event *domain.WalletLinkedEvent) bool {
		return event.UserID == req.UserId &&
			event.AccountID == req.AccountId &&
			event.Address == req.Address &&
			event.ChainID == req.ChainId &&
			event.IsPrimary == req.IsPrimary
	})).Return(nil)

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
	// Note: Event publishing is async, so we might not assert it immediately
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_Success_ExistingWallet() {
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
		CreatedAt:  time.Now().Add(-1 * time.Hour),
		UpdatedAt:  time.Now(),
		VerifiedAt: &time.Time{},
	}

	expectedResult := &domain.WalletUpsertResult{
		Link:           walletLink,
		Created:        false,
		PrimaryChanged: false,
	}

	// Mock service call
	suite.mockService.On("UpsertLink", ctx, mock.AnythingOfType("domain.WalletLink")).
		Return(expectedResult, nil)

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
	}

	suite.mockService.On("UpsertLink", ctx, mock.AnythingOfType("domain.WalletLink")).
		Return((*domain.WalletUpsertResult)(nil), domain.ErrInvalidAddress)

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
	}

	suite.mockService.On("UpsertLink", ctx, mock.AnythingOfType("domain.WalletLink")).
		Return((*domain.WalletUpsertResult)(nil), domain.ErrUnauthorizedAccess)

	resp, err := suite.handler.UpsertLink(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	suite.Equal(codes.PermissionDenied, st.Code())
	suite.Contains(st.Message(), "unauthorized_access")
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *WalletGRPCTestSuite) TestUpsertLink_EventPublishingFailure() {
	ctx := context.Background()
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: true,
	}

	walletLink := &domain.WalletLink{
		ID:         "wallet-789",
		UserID:     req.UserId,
		AccountID:  req.AccountId,
		Address:    req.Address,
		ChainID:    req.ChainId,
		IsPrimary:  req.IsPrimary,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		VerifiedAt: &time.Time{},
	}

	expectedResult := &domain.WalletUpsertResult{
		Link:           walletLink,
		Created:        true,
		PrimaryChanged: true,
	}

	// Mock service call
	suite.mockService.On("UpsertLink", ctx, mock.AnythingOfType("domain.WalletLink")).
		Return(expectedResult, nil)

	// Mock event publishing failure - should not fail the response
	suite.mockPublisher.On("PublishWalletLinked", mock.Anything, mock.AnythingOfType("*domain.WalletLinkedEvent")).
		Return(assert.AnError)

	resp, err := suite.handler.UpsertLink(ctx, req)

	// Response should still succeed even if event publishing fails
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
	mockPublisher := new(MockEventPublisher)
	handler := grpcHandler.NewWalletGRPCServer(mockService, mockPublisher)
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
			ID:        "wallet-789",
			UserID:    req.UserId,
			AccountID: req.AccountId,
			Address:   req.Address,
			ChainID:   req.ChainId,
			IsPrimary: req.IsPrimary,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		expectedResult := &domain.WalletUpsertResult{
			Link:           walletLink,
			Created:        false,
			PrimaryChanged: false,
		}

		mockService.On("UpsertLink", ctx, mock.AnythingOfType("domain.WalletLink")).
			Return(expectedResult, nil).Once()

		resp, err := handler.UpsertLink(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, walletLink.ID, resp.Link.Id)
		assert.False(t, resp.Created)
		assert.False(t, resp.PrimaryChanged)
	})
}

// Test domain to proto conversion
func TestDomainToProtoConversion(t *testing.T) {
	now := time.Now()
	verifiedAt := now.Add(-1 * time.Hour)

	domainLink := &domain.WalletLink{
		ID:         "wallet-123",
		UserID:     "user-456",
		AccountID:  "account-789",
		Address:    "0x1234567890123456789012345678901234567890",
		ChainID:    "eip155:1",
		IsPrimary:  true,
		CreatedAt:  now,
		UpdatedAt:  now,
		VerifiedAt: &verifiedAt,
	}

	mockService := new(MockWalletService)
	mockPublisher := new(MockEventPublisher)
	handler := grpcHandler.NewWalletGRPCServer(mockService, mockPublisher)

	// Test the domain to proto conversion method
	protoLink := handler.DomainLinkToProto(domainLink)

	assert.Equal(t, domainLink.ID, protoLink.Id)
	assert.Equal(t, domainLink.UserID, protoLink.UserId)
	assert.Equal(t, domainLink.AccountID, protoLink.AccountId)
	assert.Equal(t, domainLink.Address, protoLink.Address)
	assert.Equal(t, domainLink.ChainID, protoLink.ChainId)
	assert.Equal(t, domainLink.IsPrimary, protoLink.IsPrimary)
	assert.NotNil(t, protoLink.CreatedAt)
	assert.NotNil(t, protoLink.UpdatedAt)
	assert.NotNil(t, protoLink.VerifiedAt)
}

// Test proto to domain conversion
func TestProtoToDomainConversion(t *testing.T) {
	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: true,
	}

	mockService := new(MockWalletService)
	mockPublisher := new(MockEventPublisher)
	handler := grpcHandler.NewWalletGRPCServer(mockService, mockPublisher)

	// Test the proto to domain conversion method
	domainLink := handler.RequestToDomain(req)

	assert.Equal(t, req.UserId, domainLink.UserID)
	assert.Equal(t, req.AccountId, domainLink.AccountID)
	assert.Equal(t, req.Address, domainLink.Address)
	assert.Equal(t, req.ChainId, domainLink.ChainID)
	assert.Equal(t, req.IsPrimary, domainLink.IsPrimary)
	// ID and timestamps should be empty/zero for new requests
	assert.Empty(t, domainLink.ID)
}

// Benchmark tests for gRPC handlers
func BenchmarkUpsertLink(b *testing.B) {
	mockService := new(MockWalletService)
	mockPublisher := new(MockEventPublisher)
	handler := grpcHandler.NewWalletGRPCServer(mockService, mockPublisher)
	ctx := context.Background()

	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: false,
	}

	walletLink := &domain.WalletLink{
		ID:        "wallet-789",
		UserID:    req.UserId,
		AccountID: req.AccountId,
		Address:   req.Address,
		ChainID:   req.ChainId,
		IsPrimary: req.IsPrimary,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	expectedResult := &domain.WalletUpsertResult{
		Link:           walletLink,
		Created:        false,
		PrimaryChanged: false,
	}

	mockService.On("UpsertLink", mock.Anything, mock.Anything).
		Return(expectedResult, nil)

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
	mockPublisher := new(MockEventPublisher)
	handler := grpcHandler.NewWalletGRPCServer(mockService, mockPublisher)
	ctx := context.Background()

	req := &walletpb.UpsertLinkRequest{
		UserId:    "user-123",
		AccountId: "account-456",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
		IsPrimary: false,
	}

	walletLink := &domain.WalletLink{
		ID:        "wallet-789",
		UserID:    req.UserId,
		AccountID: req.AccountId,
		Address:   req.Address,
		ChainID:   req.ChainId,
		IsPrimary: req.IsPrimary,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	expectedResult := &domain.WalletUpsertResult{
		Link:           walletLink,
		Created:        false,
		PrimaryChanged: false,
	}

	mockService.On("UpsertLink", mock.Anything, mock.Anything).
		Return(expectedResult, nil)

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
			expectedCode: codes.NotFound,
			expectedMsg:  "wallet_not_found",
		},
		{
			name:         "unauthorized_access",
			serviceError: domain.ErrUnauthorizedAccess,
			expectedCode: codes.PermissionDenied,
			expectedMsg:  "unauthorized_access",
		},
		{
			name:         "invalid_address",
			serviceError: domain.ErrInvalidAddress,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid_address",
		},
		{
			name:         "invalid_chain_id",
			serviceError: domain.ErrInvalidChainID,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid_chain_id",
		},
		{
			name:         "generic_error",
			serviceError: assert.AnError,
			expectedCode: codes.Internal,
			expectedMsg:  "internal server error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockService := new(MockWalletService)
			mockPublisher := new(MockEventPublisher)
			handler := grpcHandler.NewWalletGRPCServer(mockService, mockPublisher)
			ctx := context.Background()

			req := &walletpb.UpsertLinkRequest{
				UserId:    "user-123",
				AccountId: "account-456",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainId:   "eip155:1",
				IsPrimary: false,
			}

			mockService.On("UpsertLink", ctx, mock.AnythingOfType("domain.WalletLink")).
				Return((*domain.WalletUpsertResult)(nil), tc.serviceError)

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
