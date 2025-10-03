package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"
	grpcHandler "github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/infrastructure/grpc"
	authpb "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
)

// MockAuthService is a mock implementation of AuthService
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) GetNonce(ctx context.Context, accountID, chainID, domain string) (string, error) {
	args := m.Called(ctx, accountID, chainID, domain)
	return args.String(0), args.Error(1)
}

func (m *MockAuthService) VerifySiwe(ctx context.Context, accountID, message, signature string) (*domain.AuthResult, error) {
	args := m.Called(ctx, accountID, message, signature)
	return args.Get(0).(*domain.AuthResult), args.Error(1)
}

func (m *MockAuthService) Refresh(ctx context.Context, refreshToken string) (*domain.AuthResult, error) {
	args := m.Called(ctx, refreshToken)
	return args.Get(0).(*domain.AuthResult), args.Error(1)
}

func (m *MockAuthService) Logout(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAuthService) LogoutByRefreshToken(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
	return args.Error(0)
}

// AuthGRPCTestSuite defines the test suite for Auth gRPC handler
type AuthGRPCTestSuite struct {
	suite.Suite
	handler interface {
		GetNonce(context.Context, *authpb.GetNonceRequest) (*authpb.GetNonceResponse, error)
		VerifySiwe(context.Context, *authpb.VerifySiweRequest) (*authpb.VerifySiweResponse, error)
		RefreshSession(context.Context, *authpb.RefreshSessionRequest) (*authpb.RefreshSessionResponse, error)
		RevokeSession(context.Context, *authpb.RevokeSessionRequest) (*authpb.RevokeSessionResponse, error)
		RevokeSessionByRefreshToken(context.Context, *authpb.RevokeSessionByRefreshTokenRequest) (*authpb.RevokeSessionByRefreshTokenResponse, error)
	}
	mockService *MockAuthService
}

func (suite *AuthGRPCTestSuite) SetupTest() {
	suite.mockService = new(MockAuthService)
	server := grpc.NewServer()
	suite.handler = grpcHandler.NewgRPCHandler(server, suite.mockService)
}

func (suite *AuthGRPCTestSuite) TestGetNonce_Success() {
	ctx := context.Background()
	req := &authpb.GetNonceRequest{
		AccountId: "test-account",
		ChainId:   "eip155:1",
		Domain:    "localhost",
	}

	expectedNonce := "test-nonce-123"

	suite.mockService.On("GetNonce", ctx, req.AccountId, req.ChainId, req.Domain).
		Return(expectedNonce, nil)

	resp, err := suite.handler.GetNonce(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(expectedNonce, resp.Nonce)
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *AuthGRPCTestSuite) TestGetNonce_InvalidRequest() {
	ctx := context.Background()

	testCases := []struct {
		name string
		req  *authpb.GetNonceRequest
	}{
		{
			name: "missing account_id",
			req: &authpb.GetNonceRequest{
				ChainId: "eip155:1",
				Domain:  "localhost",
			},
		},
		{
			name: "missing chain_id",
			req: &authpb.GetNonceRequest{
				AccountId: "test-account",
				Domain:    "localhost",
			},
		},
		{
			name: "missing domain",
			req: &authpb.GetNonceRequest{
				AccountId: "test-account",
				ChainId:   "eip155:1",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			resp, err := suite.handler.GetNonce(ctx, tc.req)

			assert.Error(t, err)
			assert.Nil(t, resp)

			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
		})
	}
}

func (suite *AuthGRPCTestSuite) TestVerifySiwe_Success() {
	ctx := context.Background()
	req := &authpb.VerifySiweRequest{
		AccountId: "test-account",
		Message:   "test message",
		Signature: "0x1234567890abcdef",
	}

	expectedResult := &domain.AuthResult{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-123",
		UserID:       "user-123",
		ExpiresAt:    time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
		Address:      "0x1234567890123456789012345678901234567890",
		ChainID:      "eip155:1",
	}

	suite.mockService.On("VerifySiwe", ctx, req.AccountId, req.Message, req.Signature).
		Return(expectedResult, nil)

	resp, err := suite.handler.VerifySiwe(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(expectedResult.AccessToken, resp.AccessToken)
	suite.Equal(expectedResult.RefreshToken, resp.RefreshToken)
	suite.Equal(expectedResult.UserID, resp.UserId)
	suite.Equal(expectedResult.ExpiresAt.Format(time.RFC3339), resp.ExpiresAt)
	suite.Equal(expectedResult.Address, resp.Address)
	suite.Equal(expectedResult.ChainID, resp.ChainId)
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *AuthGRPCTestSuite) TestVerifySiwe_InvalidRequest() {
	ctx := context.Background()

	testCases := []struct {
		name string
		req  *authpb.VerifySiweRequest
	}{
		{
			name: "missing account_id",
			req: &authpb.VerifySiweRequest{
				Message:   "test message",
				Signature: "0x1234567890abcdef",
			},
		},
		{
			name: "missing message",
			req: &authpb.VerifySiweRequest{
				AccountId: "test-account",
				Signature: "0x1234567890abcdef",
			},
		},
		{
			name: "missing signature",
			req: &authpb.VerifySiweRequest{
				AccountId: "test-account",
				Message:   "test message",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			resp, err := suite.handler.VerifySiwe(ctx, tc.req)

			assert.Error(t, err)
			assert.Nil(t, resp)

			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
		})
	}
}

func (suite *AuthGRPCTestSuite) TestVerifySiwe_ServiceError() {
	ctx := context.Background()
	req := &authpb.VerifySiweRequest{
		AccountId: "test-account",
		Message:   "invalid message",
		Signature: "0xinvalid",
	}

	suite.mockService.On("VerifySiwe", ctx, req.AccountId, req.Message, req.Signature).
		Return((*domain.AuthResult)(nil), errors.New("invalid signature"))

	resp, err := suite.handler.VerifySiwe(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	// Handler maps service error to Internal for VerifySiwe
	suite.Equal(codes.Internal, st.Code())
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *AuthGRPCTestSuite) TestRefreshSession_Success() {
	ctx := context.Background()
	req := &authpb.RefreshSessionRequest{
		RefreshToken: "valid-refresh-token",
		UserAgent:    "test-agent",
		IpAddress:    "127.0.0.1",
	}

	expectedResult := &domain.AuthResult{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		UserID:       "user-123",
		ExpiresAt:    time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
	}

	suite.mockService.On("Refresh", ctx, req.RefreshToken).
		Return(expectedResult, nil)

	resp, err := suite.handler.RefreshSession(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(expectedResult.AccessToken, resp.AccessToken)
	suite.Equal(expectedResult.RefreshToken, resp.RefreshToken)
	suite.Equal(expectedResult.UserID, resp.UserId)
	suite.Equal(expectedResult.ExpiresAt.Format(time.RFC3339), resp.ExpiresAt)
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *AuthGRPCTestSuite) TestRefreshSession_InvalidToken() {
	ctx := context.Background()
	req := &authpb.RefreshSessionRequest{
		RefreshToken: "invalid-refresh-token",
		UserAgent:    "test-agent",
		IpAddress:    "127.0.0.1",
	}

	suite.mockService.On("Refresh", ctx, req.RefreshToken).
		Return((*domain.AuthResult)(nil), errors.New("session not found"))

	resp, err := suite.handler.RefreshSession(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	suite.Equal(codes.Unauthenticated, st.Code())
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *AuthGRPCTestSuite) TestRevokeSession_Success() {
	ctx := context.Background()
	req := &authpb.RevokeSessionRequest{
		SessionId: "session-123",
	}

	suite.mockService.On("Logout", ctx, req.SessionId).Return(nil)

	resp, err := suite.handler.RevokeSession(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.mockService.AssertExpectations(suite.T())
}

func (suite *AuthGRPCTestSuite) TestRevokeSession_InvalidRequest() {
	ctx := context.Background()
	req := &authpb.RevokeSessionRequest{
		SessionId: "",
	}

	resp, err := suite.handler.RevokeSession(ctx, req)

	suite.Error(err)
	suite.Nil(resp)

	st, ok := status.FromError(err)
	suite.True(ok)
	suite.Equal(codes.InvalidArgument, st.Code())
}

func (suite *AuthGRPCTestSuite) TestRevokeSessionByRefreshToken_Success() {
	ctx := context.Background()
	req := &authpb.RevokeSessionByRefreshTokenRequest{
		RefreshToken: "refresh-token-123",
	}

	suite.mockService.On("LogoutByRefreshToken", ctx, req.RefreshToken).Return(nil)

	resp, err := suite.handler.RevokeSessionByRefreshToken(ctx, req)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.mockService.AssertExpectations(suite.T())
}

func TestAuthGRPCTestSuite(t *testing.T) {
	suite.Run(t, new(AuthGRPCTestSuite))
}

// Additional unit tests for edge cases
func TestGRPCValidation(t *testing.T) {
	handler := grpcHandler.NewgRPCHandler(grpc.NewServer(), new(MockAuthService))
	ctx := context.Background()

	t.Run("EmptyStrings", func(t *testing.T) {
		req := &authpb.GetNonceRequest{
			AccountId: "",
			ChainId:   "",
			Domain:    "",
		}
		resp, err := handler.GetNonce(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

// Benchmark tests for gRPC handlers
func BenchmarkGetNonce(b *testing.B) {
	mockService := new(MockAuthService)
	handler := grpcHandler.NewgRPCHandler(grpc.NewServer(), mockService)
	ctx := context.Background()

	req := &authpb.GetNonceRequest{
		AccountId: "test-account",
		ChainId:   "eip155:1",
		Domain:    "localhost",
	}

	expectedNonce := "test-nonce"
	mockService.On("GetNonce", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(expectedNonce, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.GetNonce(ctx, req)
	}
}

func BenchmarkVerifySiwe(b *testing.B) {
	mockService := new(MockAuthService)
	handler := grpcHandler.NewgRPCHandler(grpc.NewServer(), mockService)
	ctx := context.Background()

	req := &authpb.VerifySiweRequest{
		AccountId: "test-account",
		Message:   "test message",
		Signature: "0x1234567890abcdef",
	}

	expectedResult := &domain.AuthResult{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		UserID:       "user-123",
		ExpiresAt:    time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
	}
	mockService.On("VerifySiwe", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(expectedResult, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.VerifySiwe(ctx, req)
	}
}
