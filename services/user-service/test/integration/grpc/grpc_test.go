package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
	grpcHandler "github.com/quangdang46/NFT-Marketplace/services/user-service/internal/infrastructure/grpc"
	userpb "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
)

// MockUserService is a mock implementation of UserService
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) EnsureUser(ctx context.Context, accountID, address, chainID string) (domain.UserID, bool, error) {
	args := m.Called(ctx, accountID, address, chainID)
	return args.Get(0).(domain.UserID), args.Bool(1), args.Error(2)
}

func (m *MockUserService) GetUser(ctx context.Context, userID domain.UserID) (*domain.User, *domain.Profile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(*domain.User), args.Get(1).(*domain.Profile), args.Error(2)
}

func (m *MockUserService) UpdateProfile(ctx context.Context, profile *domain.Profile) (*domain.Profile, error) {
	args := m.Called(ctx, profile)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Profile), args.Error(1)
}

// UserGRPCTestSuite defines the test suite for User gRPC handler
type UserGRPCTestSuite struct {
	suite.Suite
	handler interface {
		EnsureUser(context.Context, *userpb.EnsureUserRequest) (*userpb.EnsureUserResponse, error)
	}
	mockService *MockUserService
}

func (suite *UserGRPCTestSuite) SetupTest() {
	suite.mockService = new(MockUserService)
	server := grpc.NewServer()
	suite.handler = grpcHandler.NewgRPCHandler(server, suite.mockService)
}

func (suite *UserGRPCTestSuite) TestEnsureUser_Success_NewUser() {
	ctx := context.Background()
	req := &userpb.EnsureUserRequest{
		AccountId: "test-account-123",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
	}

	expectedUserID := domain.UserID("user-789")
	expectedCreated := true

	suite.mockService.On("EnsureUser", ctx, req.AccountId, req.Address, req.ChainId).
		Return(expectedUserID, expectedCreated, nil)

	resp, err := suite.handler.EnsureUser(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(string(expectedUserID), resp.UserId)
	suite.Equal(expectedCreated, resp.Created)
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *UserGRPCTestSuite) TestEnsureUser_Success_ExistingUser() {
	ctx := context.Background()
	req := &userpb.EnsureUserRequest{
		AccountId: "test-account-123",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
	}

	expectedUserID := domain.UserID("user-456")
	expectedCreated := false

	suite.mockService.On("EnsureUser", ctx, req.AccountId, req.Address, req.ChainId).
		Return(expectedUserID, expectedCreated, nil)

	resp, err := suite.handler.EnsureUser(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(string(expectedUserID), resp.UserId)
	suite.Equal(expectedCreated, resp.Created)
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *UserGRPCTestSuite) TestEnsureUser_InvalidRequest() {
	ctx := context.Background()

	testCases := []struct {
		name string
		req  *userpb.EnsureUserRequest
	}{
		{
			name: "nil_request",
			req:  nil,
		},
		{
			name: "missing_account_id",
			req: &userpb.EnsureUserRequest{
				Address: "0x1234567890123456789012345678901234567890",
				ChainId: "eip155:1",
			},
		},
		{
			name: "missing_address",
			req: &userpb.EnsureUserRequest{
				AccountId: "test-account",
				ChainId:   "eip155:1",
			},
		},
		{
			name: "empty_account_id",
			req: &userpb.EnsureUserRequest{
				AccountId: "",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainId:   "eip155:1",
			},
		},
		{
			name: "empty_address",
			req: &userpb.EnsureUserRequest{
				AccountId: "test-account",
				Address:   "",
				ChainId:   "eip155:1",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			resp, err := suite.handler.EnsureUser(ctx, tc.req)

			assert.Error(t, err)
			assert.Nil(t, resp)

			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
		})
	}
}

func (suite *UserGRPCTestSuite) TestEnsureUser_ServiceError() {
	ctx := context.Background()
	req := &userpb.EnsureUserRequest{
		AccountId: "test-account-123",
		Address:   "invalid-address",
		ChainId:   "eip155:1",
	}

	suite.mockService.On("EnsureUser", ctx, req.AccountId, req.Address, req.ChainId).
		Return(domain.UserID(""), false, domain.ErrInvalidAddress)

	resp, err := suite.handler.EnsureUser(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	suite.Equal(codes.InvalidArgument, st.Code())
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *UserGRPCTestSuite) TestEnsureUser_DatabaseError() {
	ctx := context.Background()
	req := &userpb.EnsureUserRequest{
		AccountId: "test-account-123",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
	}

	dbError := assert.AnError
	suite.mockService.On("EnsureUser", ctx, req.AccountId, req.Address, req.ChainId).
		Return(domain.UserID(""), false, dbError)

	resp, err := suite.handler.EnsureUser(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	suite.Equal(codes.Internal, st.Code())
	suite.Contains(st.Message(), "general error")
	suite.mockService.AssertExpectations(suite.T())
}

func TestUserGRPCTestSuite(t *testing.T) {
	suite.Run(t, new(UserGRPCTestSuite))
}

// Additional unit tests for edge cases
func TestGRPCValidation(t *testing.T) {
	mockService := new(MockUserService)
	server := grpc.NewServer()
	handler := grpcHandler.NewgRPCHandler(server, mockService)
	ctx := context.Background()

	t.Run("ValidRequest", func(t *testing.T) {
		req := &userpb.EnsureUserRequest{
			AccountId: "test-account",
			Address:   "0x1234567890123456789012345678901234567890",
			ChainId:   "eip155:1",
		}

		expectedUserID := domain.UserID("user-123")
		expectedCreated := true

		mockService.On("EnsureUser", ctx, req.AccountId, req.Address, req.ChainId).
			Return(expectedUserID, expectedCreated, nil).Once()

		resp, err := handler.EnsureUser(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, string(expectedUserID), resp.UserId)
		assert.Equal(t, expectedCreated, resp.Created)
	})

	t.Run("WhitespaceOnlyFields", func(t *testing.T) {
		req := &userpb.EnsureUserRequest{
			AccountId: "   ",
			Address:   "0x1234567890123456789012345678901234567890",
			ChainId:   "eip155:1",
		}

		// This should be handled by validation if implemented
		// For now, we'll test that it reaches the service layer
		mockService.On("EnsureUser", ctx, req.AccountId, req.Address, req.ChainId).
			Return(domain.UserID(""), false, domain.ErrInvalidAddress).Once()

		resp, err := handler.EnsureUser(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// Benchmark tests for gRPC handlers
func BenchmarkEnsureUser(b *testing.B) {
	mockService := new(MockUserService)
	server := grpc.NewServer()
	handler := grpcHandler.NewgRPCHandler(server, mockService)
	ctx := context.Background()

	req := &userpb.EnsureUserRequest{
		AccountId: "test-account",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
	}

	expectedUserID := domain.UserID("user-123")
	expectedCreated := false

	mockService.On("EnsureUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(expectedUserID, expectedCreated, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.EnsureUser(ctx, req)
	}
}

// Stress tests
func TestEnsureUser_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	mockService := new(MockUserService)
	server := grpc.NewServer()
	handler := grpcHandler.NewgRPCHandler(server, mockService)
	ctx := context.Background()

	req := &userpb.EnsureUserRequest{
		AccountId: "test-account",
		Address:   "0x1234567890123456789012345678901234567890",
		ChainId:   "eip155:1",
	}

	expectedUserID := domain.UserID("user-123")
	expectedCreated := false

	mockService.On("EnsureUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(expectedUserID, expectedCreated, nil)

	// Run concurrent requests
	concurrency := 10
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer func() { done <- true }()

			resp, err := handler.EnsureUser(ctx, req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, string(expectedUserID), resp.UserId)
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
			name:         "invalid_input_error",
			serviceError: domain.ErrInvalidAddress,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid address",
		},
		{
			name:         "invalid_address_error",
			serviceError: domain.ErrInvalidAddress,
			expectedCode: codes.InvalidArgument,
			expectedMsg:  "invalid address",
		},
		{
			name:         "database_error",
			serviceError: domain.ErrUserNotFound,
			expectedCode: codes.Internal,
			expectedMsg:  "user not found",
		},
		{
			name:         "generic_error",
			serviceError: assert.AnError,
			expectedCode: codes.Internal,
			expectedMsg:  "assert.AnError general error for testing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockService := new(MockUserService)
			server := grpc.NewServer()
			handler := grpcHandler.NewgRPCHandler(server, mockService)
			ctx := context.Background()

			req := &userpb.EnsureUserRequest{
				AccountId: "test-account",
				Address:   "0x1234567890123456789012345678901234567890",
				ChainId:   "eip155:1",
			}

			mockService.On("EnsureUser", ctx, req.AccountId, req.Address, req.ChainId).
				Return(domain.UserID(""), false, tc.serviceError)

			resp, err := handler.EnsureUser(ctx, req)

			assert.Error(t, err)
			assert.Nil(t, resp)

			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, tc.expectedCode, st.Code())
			assert.Contains(t, st.Message(), tc.expectedMsg)
		})
	}
}
