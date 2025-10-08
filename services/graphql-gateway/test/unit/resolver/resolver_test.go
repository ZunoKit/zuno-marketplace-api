package test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	graphql_resolver "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/graphql/schemas"
	grpcclients "github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/grpc_clients"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/middleware"
	authpb "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	walletpb "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"google.golang.org/grpc"
)

// MockAuthServiceClient is a mock implementation of AuthServiceClient
type MockAuthServiceClient struct {
	mock.Mock
}

func (m *MockAuthServiceClient) GetNonce(ctx context.Context, req *authpb.GetNonceRequest, opts ...grpc.CallOption) (*authpb.GetNonceResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*authpb.GetNonceResponse), args.Error(1)
}

func (m *MockAuthServiceClient) VerifySiwe(ctx context.Context, req *authpb.VerifySiweRequest, opts ...grpc.CallOption) (*authpb.VerifySiweResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*authpb.VerifySiweResponse), args.Error(1)
}

func (m *MockAuthServiceClient) RefreshSession(ctx context.Context, req *authpb.RefreshSessionRequest, opts ...grpc.CallOption) (*authpb.RefreshSessionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*authpb.RefreshSessionResponse), args.Error(1)
}

func (m *MockAuthServiceClient) RevokeSession(ctx context.Context, req *authpb.RevokeSessionRequest, opts ...grpc.CallOption) (*authpb.RevokeSessionResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*authpb.RevokeSessionResponse), args.Error(1)
}

func (m *MockAuthServiceClient) RevokeSessionByRefreshToken(ctx context.Context, req *authpb.RevokeSessionByRefreshTokenRequest, opts ...grpc.CallOption) (*authpb.RevokeSessionByRefreshTokenResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*authpb.RevokeSessionByRefreshTokenResponse), args.Error(1)
}

// MockWalletServiceClient is a mock implementation of WalletServiceClient
type MockWalletServiceClient struct {
	mock.Mock
}

func (m *MockWalletServiceClient) UpsertLink(ctx context.Context, req *walletpb.UpsertLinkRequest, opts ...grpc.CallOption) (*walletpb.UpsertLinkResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*walletpb.UpsertLinkResponse), args.Error(1)
}

// MockCollectionServiceClient is a mock implementation of CollectionServiceClient

// ResolverTestSuite defines the test suite for GraphQL resolvers
type ResolverTestSuite struct {
	suite.Suite
	resolver         *graphql_resolver.Resolver
	mockAuthClient   *MockAuthServiceClient
	mockWalletClient *MockWalletServiceClient
	mutationResolver schemas.MutationResolver
	queryResolver    schemas.QueryResolver
}

func (suite *ResolverTestSuite) SetupTest() {
	suite.mockAuthClient = new(MockAuthServiceClient)
	suite.mockWalletClient = new(MockWalletServiceClient)

	// Create mock gRPC clients (interfaces addressed)
	var ac authpb.AuthServiceClient = suite.mockAuthClient
	var wc walletpb.WalletServiceClient = suite.mockWalletClient
	authClient := &grpcclients.AuthClient{Client: &ac}
	walletClient := &grpcclients.WalletClient{Client: &wc}

	suite.resolver = graphql_resolver.NewResolver(authClient, walletClient, nil)
	suite.mutationResolver = suite.resolver.Mutation()
	suite.queryResolver = suite.resolver.Query()
}

func (suite *ResolverTestSuite) TestSignInSiwe_Success() {
	ctx := context.Background()
	input := schemas.SignInSiweInput{
		AccountID: "test-account-123",
		ChainID:   "eip155:1",
		Domain:    "localhost",
	}

	expectedResponse := &authpb.GetNonceResponse{
		Nonce: "test-nonce-456",
	}

	suite.mockAuthClient.On("GetNonce", ctx, &authpb.GetNonceRequest{
		AccountId: input.AccountID,
		ChainId:   input.ChainID,
		Domain:    input.Domain,
	}).Return(expectedResponse, nil)

	result, err := suite.mutationResolver.SignInSiwe(ctx, input)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse.Nonce, result.Nonce)
	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestSignInSiwe_InvalidInput() {
	ctx := context.Background()

	testCases := []struct {
		name  string
		input schemas.SignInSiweInput
	}{
		{
			name: "empty_account_id",
			input: schemas.SignInSiweInput{
				AccountID: "",
				ChainID:   "eip155:1",
				Domain:    "localhost",
			},
		},
		{
			name: "empty_chain_id",
			input: schemas.SignInSiweInput{
				AccountID: "test-account",
				ChainID:   "",
				Domain:    "localhost",
			},
		},
		{
			name: "empty_domain",
			input: schemas.SignInSiweInput{
				AccountID: "test-account",
				ChainID:   "eip155:1",
				Domain:    "",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.mutationResolver.SignInSiwe(ctx, tc.input)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "invalid sign in siwe input")
		})
	}
}

func (suite *ResolverTestSuite) TestVerifySiwe_Success() {
	ctx := context.Background()
	input := schemas.VerifySiweInput{
		AccountID: "test-account-123",
		Message:   "test message",
		Signature: "0x1234567890abcdef",
	}

	expectedResponse := &authpb.VerifySiweResponse{
		AccessToken:  "access-token-123",
		RefreshToken: "refresh-token-456",
		ExpiresAt:    "2023-12-31T23:59:59Z",
		UserId:       "user-789",
		Address:      "0x1234567890123456789012345678901234567890",
		ChainId:      "eip155:1",
	}

	suite.mockAuthClient.On("VerifySiwe", ctx, &authpb.VerifySiweRequest{
		AccountId: input.AccountID,
		Message:   input.Message,
		Signature: input.Signature,
	}).Return(expectedResponse, nil)

	result, err := suite.mutationResolver.VerifySiwe(ctx, input)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse.AccessToken, result.AccessToken)
	suite.Equal(expectedResponse.RefreshToken, result.RefreshToken)
	suite.Equal(expectedResponse.ExpiresAt, result.ExpiresAt)
	suite.Equal(expectedResponse.UserId, result.UserID)
	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestVerifySiwe_InvalidInput() {
	ctx := context.Background()

	testCases := []struct {
		name  string
		input schemas.VerifySiweInput
	}{
		{
			name: "empty_account_id",
			input: schemas.VerifySiweInput{
				AccountID: "",
				Message:   "test message",
				Signature: "0x1234567890abcdef",
			},
		},
		{
			name: "empty_message",
			input: schemas.VerifySiweInput{
				AccountID: "test-account",
				Message:   "",
				Signature: "0x1234567890abcdef",
			},
		},
		{
			name: "empty_signature",
			input: schemas.VerifySiweInput{
				AccountID: "test-account",
				Message:   "test message",
				Signature: "",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := suite.mutationResolver.VerifySiwe(ctx, tc.input)

			assert.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "invalid verify siwe input")
		})
	}
}

func (suite *ResolverTestSuite) TestRefreshSession_Success() {
	ctx := context.Background()

	// Mock HTTP request in context with refresh token cookie
	ctx = suite.addRefreshTokenToContext(ctx, "valid-refresh-token")

	expectedResponse := &authpb.RefreshSessionResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresAt:    "2023-12-31T23:59:59Z",
		UserId:       "user-789",
	}

	suite.mockAuthClient.On("RefreshSession", ctx, mock.MatchedBy(func(req *authpb.RefreshSessionRequest) bool {
		return req.RefreshToken == "valid-refresh-token"
	})).Return(expectedResponse, nil)

	result, err := suite.mutationResolver.RefreshSession(ctx)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse.AccessToken, result.AccessToken)
	suite.Equal(expectedResponse.RefreshToken, result.RefreshToken)
	suite.Equal(expectedResponse.ExpiresAt, result.ExpiresAt)
	suite.Equal(expectedResponse.UserId, result.UserID)
	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestRefreshSession_NoRefreshToken() {
	ctx := context.Background()

	// Context without refresh token
	ctx = suite.addEmptyRequestToContext(ctx)

	result, err := suite.mutationResolver.RefreshSession(ctx)

	suite.Error(err)
	suite.Nil(result)
	suite.Contains(err.Error(), "refresh token not found")
}

func (suite *ResolverTestSuite) TestLogout_Success() {
	ctx := context.Background()

	// Mock authenticated user in context
	user := &middleware.CurrentUser{
		UserID:    "user-123",
		SessionID: "session-456",
	}
	ctx = suite.addUserToContext(ctx, user)
	ctx = suite.addRefreshTokenToContext(ctx, "refresh-token-789")

	suite.mockAuthClient.On("RevokeSession", ctx, &authpb.RevokeSessionRequest{
		SessionId: user.SessionID,
	}).Return(&authpb.RevokeSessionResponse{}, nil)

	suite.mockAuthClient.On("RevokeSessionByRefreshToken", ctx, &authpb.RevokeSessionByRefreshTokenRequest{
		RefreshToken: "refresh-token-789",
	}).Return(&authpb.RevokeSessionByRefreshTokenResponse{}, nil)

	result, err := suite.mutationResolver.Logout(ctx)

	suite.NoError(err)
	suite.True(result)
	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestLogout_UnauthenticatedUser() {
	ctx := context.Background()

	// Context without authenticated user but with refresh token
	ctx = suite.addRefreshTokenToContext(ctx, "refresh-token-789")

	suite.mockAuthClient.On("RevokeSessionByRefreshToken", ctx, &authpb.RevokeSessionByRefreshTokenRequest{
		RefreshToken: "refresh-token-789",
	}).Return(&authpb.RevokeSessionByRefreshTokenResponse{}, nil)

	result, err := suite.mutationResolver.Logout(ctx)

	suite.NoError(err)
	suite.True(result)
	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestUpdateProfile_Success() {
	ctx := context.Background()
	displayName := "New Display Name"

	// Mock authenticated user in context
	user := &middleware.CurrentUser{
		UserID:    "user-123",
		SessionID: "session-456",
	}
	ctx = suite.addUserToContext(ctx, user)

	result, err := suite.mutationResolver.UpdateProfile(ctx, &displayName)

	suite.NoError(err)
	suite.True(result)
}

func (suite *ResolverTestSuite) TestUpdateProfile_Unauthenticated() {
	ctx := context.Background()
	displayName := "New Display Name"

	// Context without authenticated user
	result, err := suite.mutationResolver.UpdateProfile(ctx, &displayName)

	suite.Error(err)
	suite.False(result)
	suite.Contains(err.Error(), "authentication required")
}

func (suite *ResolverTestSuite) TestHealth_Success() {
	ctx := context.Background()

	result, err := suite.queryResolver.Health(ctx)

	suite.NoError(err)
	suite.Equal("ok", result)
}

func (suite *ResolverTestSuite) TestMe_AuthenticatedUser() {
	ctx := context.Background()

	// Mock authenticated user in context
	user := &middleware.CurrentUser{
		UserID:    "user-123",
		SessionID: "session-456",
	}
	ctx = suite.addUserToContext(ctx, user)

	result, err := suite.queryResolver.Me(ctx)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(user.UserID, result.ID)
}

func (suite *ResolverTestSuite) TestMe_SilentRefresh() {
	ctx := context.Background()

	// Context with refresh token but no authenticated user
	ctx = suite.addRefreshTokenToContext(ctx, "valid-refresh-token")

	expectedResponse := &authpb.RefreshSessionResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresAt:    "2023-12-31T23:59:59Z",
		UserId:       "user-789",
	}

	suite.mockAuthClient.On("RefreshSession", ctx, mock.MatchedBy(func(req *authpb.RefreshSessionRequest) bool {
		return req.RefreshToken == "valid-refresh-token"
	})).Return(expectedResponse, nil)

	result, err := suite.queryResolver.Me(ctx)

	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(expectedResponse.UserId, result.ID)
	suite.mockAuthClient.AssertExpectations(suite.T())
}

func (suite *ResolverTestSuite) TestMe_NoAuthentication() {
	ctx := context.Background()

	// Context without authentication or refresh token
	ctx = suite.addEmptyRequestToContext(ctx)

	result, err := suite.queryResolver.Me(ctx)

	suite.NoError(err)
	suite.Nil(result)
}

func TestResolverTestSuite(t *testing.T) {
	suite.Run(t, new(ResolverTestSuite))
}

// Helper methods for setting up context
func (suite *ResolverTestSuite) addUserToContext(ctx context.Context, user *middleware.CurrentUser) context.Context {
	return context.WithValue(ctx, middleware.CurrentUserKey, user)
}

func (suite *ResolverTestSuite) addRefreshTokenToContext(ctx context.Context, refreshToken string) context.Context {
	// Create a mock HTTP request with refresh token cookie
	req := httptest.NewRequest("POST", "/graphql", nil)
	req.AddCookie(&http.Cookie{
		Name:  "refresh_token",
		Value: refreshToken,
	})

	// Add request to context
	ctx = context.WithValue(ctx, middleware.RequestKey, req)
	return ctx
}

func (suite *ResolverTestSuite) addEmptyRequestToContext(ctx context.Context) context.Context {
	// Create a mock HTTP request without cookies
	req := httptest.NewRequest("POST", "/graphql", nil)
	ctx = context.WithValue(ctx, middleware.RequestKey, req)
	return ctx
}

// Additional unit tests for error handling
func TestResolverErrorHandling(t *testing.T) {
	mockAuthClient := new(MockAuthServiceClient)
	var ac authpb.AuthServiceClient = mockAuthClient
	authClient := &grpcclients.AuthClient{Client: &ac}
	resolver := graphql_resolver.NewResolver(authClient, nil, nil)
	mutationResolver := resolver.Mutation()

	ctx := context.Background()

	t.Run("AuthServiceUnavailable", func(t *testing.T) {
		// Test with nil auth client
		resolver := graphql_resolver.NewResolver(nil, nil, nil)
		mutationResolver := resolver.Mutation()

		input := schemas.SignInSiweInput{
			AccountID: "test-account",
			ChainID:   "eip155:1",
			Domain:    "localhost",
		}

		result, err := mutationResolver.SignInSiwe(ctx, input)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "auth service unavailable")
	})

	t.Run("AuthServiceError", func(t *testing.T) {
		input := schemas.SignInSiweInput{
			AccountID: "test-account",
			ChainID:   "eip155:1",
			Domain:    "localhost",
		}

		mockAuthClient.On("GetNonce", ctx, mock.AnythingOfType("*auth.GetNonceRequest")).
			Return((*authpb.GetNonceResponse)(nil), assert.AnError)

		result, err := mutationResolver.SignInSiwe(ctx, input)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// Benchmark tests
func BenchmarkSignInSiwe(b *testing.B) {
	mockAuthClient := new(MockAuthServiceClient)
	var ac authpb.AuthServiceClient = mockAuthClient
	authClient := &grpcclients.AuthClient{Client: &ac}
	resolver := graphql_resolver.NewResolver(authClient, nil, nil)
	mutationResolver := resolver.Mutation()

	ctx := context.Background()
	input := schemas.SignInSiweInput{
		AccountID: "test-account",
		ChainID:   "eip155:1",
		Domain:    "localhost",
	}

	expectedResponse := &authpb.GetNonceResponse{Nonce: "test-nonce"}
	mockAuthClient.On("GetNonce", mock.Anything, mock.Anything).
		Return(expectedResponse, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mutationResolver.SignInSiwe(ctx, input)
	}
}

func BenchmarkVerifySiwe(b *testing.B) {
	mockAuthClient := new(MockAuthServiceClient)
	var ac authpb.AuthServiceClient = mockAuthClient
	authClient := &grpcclients.AuthClient{Client: &ac}
	resolver := graphql_resolver.NewResolver(authClient, nil, nil)
	mutationResolver := resolver.Mutation()

	ctx := context.Background()
	input := schemas.VerifySiweInput{
		AccountID: "test-account",
		Message:   "test message",
		Signature: "0x1234567890abcdef",
	}

	expectedResponse := &authpb.VerifySiweResponse{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    "2023-12-31T23:59:59Z",
		UserId:       "user-123",
	}
	mockAuthClient.On("VerifySiwe", mock.Anything, mock.Anything).
		Return(expectedResponse, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mutationResolver.VerifySiwe(ctx, input)
	}
}

// Integration-style tests
func TestResolverIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	t.Run("CompleteAuthFlow", func(t *testing.T) {
		// Test complete authentication flow: signInSiwe -> verifySiwe -> me -> logout
		t.Skip("Requires integration test setup")
	})

	t.Run("SessionRefreshFlow", func(t *testing.T) {
		// Test session refresh flow: authenticate -> wait for expiry -> refresh -> use
		t.Skip("Requires integration test setup")
	})
}

// Test concurrent resolver operations
func TestConcurrentResolverOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent test in short mode")
	}

	// Test concurrent resolver calls
	t.Skip("Requires concurrent test implementation")
}
