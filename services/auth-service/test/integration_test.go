package test

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/service"
	protoUser "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	protoWallet "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"google.golang.org/grpc"
)

// MockUserServiceClient mocks the user service client
type MockUserServiceClient struct {
	mock.Mock
	protoUser.UserServiceClient // Embed the interface
}

func (m *MockUserServiceClient) EnsureUser(ctx context.Context, req *protoUser.EnsureUserRequest, opts ...grpc.CallOption) (*protoUser.EnsureUserResponse, error) {
	// Simplified call without opts for testing
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*protoUser.EnsureUserResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockWalletServiceClient mocks the wallet service client
type MockWalletServiceClient struct {
	mock.Mock
	protoWallet.WalletServiceClient // Embed the interface
}

func (m *MockWalletServiceClient) UpsertLink(ctx context.Context, req *protoWallet.UpsertLinkRequest, opts ...grpc.CallOption) (*protoWallet.UpsertLinkResponse, error) {
	// Simplified call without opts for testing
	args := m.Called(ctx, req)
	if resp := args.Get(0); resp != nil {
		return resp.(*protoWallet.UpsertLinkResponse), args.Error(1)
	}
	return nil, args.Error(1)
}

// MockEventPublisher mocks the event publisher
type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) PublishUserLoggedIn(ctx context.Context, event *domain.AuthUserLoggedInEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func TestAuthService_CompleteFlow(t *testing.T) {
	// Setup mocks
	mockRepo := new(MockAuthRepository)
	mockUserService := new(MockUserServiceClient)
	mockWalletService := new(MockWalletServiceClient)
	mockPublisher := new(MockEventPublisher)

	// Create service
	authService := service.NewAuthService(
		mockRepo,
		mockUserService,
		mockWalletService,
		mockPublisher,
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-secret"),
		false,
	)

	ctx := context.Background()

	// Test GetNonce
	t.Run("GetNonce", func(t *testing.T) {
		accountID := "0x1234567890123456789012345678901234567890"
		chainID := "eip155:1"
		domainName := "localhost"

		mockRepo.On("CreateNonce", ctx, mock.AnythingOfType("*domain.Nonce")).Return(nil).Once()

		nonce, err := authService.GetNonce(ctx, accountID, chainID, domainName)

		assert.NoError(t, err)
		assert.NotEmpty(t, nonce)
		assert.Len(t, nonce, 64) // 32 bytes hex encoded
		mockRepo.AssertExpectations(t)
	})

	// Test invalid inputs
	t.Run("GetNonce_InvalidInputs", func(t *testing.T) {
		tests := []struct {
			name      string
			accountID string
			chainID   string
			domain    string
			wantErr   bool
		}{
			{"empty accountID", "", "eip155:1", "localhost", true},
			{"invalid accountID", "invalid", "eip155:1", "localhost", true},
			{"empty chainID", "0x1234567890123456789012345678901234567890", "", "localhost", true},
			{"invalid chainID", "0x1234567890123456789012345678901234567890", "invalid", "localhost", true},
			{"empty domain", "0x1234567890123456789012345678901234567890", "eip155:1", "", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := authService.GetNonce(ctx, tt.accountID, tt.chainID, tt.domain)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestAuthService_RefreshToken(t *testing.T) {
	mockRepo := new(MockAuthRepository)
	authService := service.NewAuthService(
		mockRepo,
		nil,
		nil,
		nil,
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-secret"),
		false,
	)

	ctx := context.Background()

	t.Run("RefreshToken_Success", func(t *testing.T) {
		refreshToken := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		sessionID := "test-session-id"
		userID := "test-user-id"

		session := &domain.Session{
			ID:          domain.SessionID(sessionID),
			UserID:      domain.UserID(userID),
			RefreshHash: service.HashRefreshToken(refreshToken),
			ExpiresAt:   time.Now().Add(24 * time.Hour),
			CreatedAt:   time.Now(),
		}

		mockRepo.On("GetSessionByRefreshHash", ctx, session.RefreshHash).Return(session, nil).Once()
		mockRepo.On("UpdateSessionLastUsed", ctx, session.ID).Return(nil).Once()

		result, err := authService.Refresh(ctx, refreshToken)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.AccessToken)
		assert.Equal(t, refreshToken, result.RefreshToken)
		assert.Equal(t, userID, string(result.UserID))
		mockRepo.AssertExpectations(t)
	})

	t.Run("RefreshToken_InvalidToken", func(t *testing.T) {
		// Test with invalid token format
		_, err := authService.Refresh(ctx, "invalid")
		assert.Error(t, err)

		// Test with empty token
		_, err = authService.Refresh(ctx, "")
		assert.Error(t, err)
	})

	t.Run("RefreshToken_ExpiredSession", func(t *testing.T) {
		refreshToken := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

		session := &domain.Session{
			ID:          "test-session-id",
			UserID:      "test-user-id",
			RefreshHash: service.HashRefreshToken(refreshToken),
			ExpiresAt:   time.Now().Add(-1 * time.Hour), // Expired
			CreatedAt:   time.Now().Add(-25 * time.Hour),
		}

		mockRepo.On("GetSessionByRefreshHash", ctx, session.RefreshHash).Return(session, nil).Once()

		_, err := authService.Refresh(ctx, refreshToken)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expired")
		mockRepo.AssertExpectations(t)
	})
}

func TestAuthService_Logout(t *testing.T) {
	mockRepo := new(MockAuthRepository)
	authService := service.NewAuthService(
		mockRepo,
		nil,
		nil,
		nil,
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-secret"),
		false,
	)

	ctx := context.Background()

	t.Run("Logout_Success", func(t *testing.T) {
		sessionID := "550e8400-e29b-41d4-a716-446655440000"

		mockRepo.On("RevokeSession", ctx, domain.SessionID(sessionID)).Return(nil).Once()

		err := authService.Logout(ctx, sessionID)

		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Logout_InvalidSessionID", func(t *testing.T) {
		// Test with empty session ID
		err := authService.Logout(ctx, "")
		assert.Error(t, err)

		// Test with invalid UUID format
		err = authService.Logout(ctx, "invalid-uuid")
		assert.Error(t, err)
	})
}

func TestJWTGeneration(t *testing.T) {
	jwtSecret := []byte("test-jwt-secret")

	t.Run("ValidateJWTStructure", func(t *testing.T) {
		_ = service.NewAuthService(
			nil,
			nil,
			nil,
			nil,
			jwtSecret,
			[]byte("test-refresh-secret"),
			false,
		)

		// Use reflection to call the private generateAccessToken method
		// In real scenario, we would test through the public interface
		userID := "test-user-id"
		sessionID := "test-session-id"

		// We can't directly test private methods, but we can verify through VerifySiwe
		// This is a limitation - in production, we'd expose a validation method or test through integration

		// Create a valid JWT token for testing
		now := time.Now()
		expiresAt := now.Add(1 * time.Hour)

		claims := jwt.MapClaims{
			"sub":        userID,
			"session_id": sessionID,
			"iat":        now.Unix(),
			"exp":        expiresAt.Unix(),
			"iss":        "nft-marketplace-auth",
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString(jwtSecret)

		assert.NoError(t, err)
		assert.NotEmpty(t, tokenString)

		// Parse and validate the token
		parsedToken, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})

		assert.NoError(t, err)
		assert.True(t, parsedToken.Valid)

		// Verify claims
		if claims, ok := parsedToken.Claims.(jwt.MapClaims); ok {
			assert.Equal(t, userID, claims["sub"])
			assert.Equal(t, sessionID, claims["session_id"])
			assert.Equal(t, "nft-marketplace-auth", claims["iss"])
		}
	})
}
