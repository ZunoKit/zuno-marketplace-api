package test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/service"
)

// MockAuthRepository is a mock implementation of domain.AuthRepository
type MockAuthRepository struct {
	mock.Mock
}

func (m *MockAuthRepository) CreateNonce(ctx context.Context, nonce *domain.Nonce) error {
	args := m.Called(ctx, nonce)
	return args.Error(0)
}

func (m *MockAuthRepository) GetNonce(ctx context.Context, value string) (*domain.Nonce, error) {
	args := m.Called(ctx, value)
	if got := args.Get(0); got != nil {
		return got.(*domain.Nonce), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) TryUseNonce(ctx context.Context, value, accountID, chainID, domainName string, usedAt time.Time) (bool, error) {
	args := m.Called(ctx, value, accountID, chainID, domainName, usedAt)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockAuthRepository) GetSession(ctx context.Context, sessionID domain.SessionID) (*domain.Session, error) {
	args := m.Called(ctx, sessionID)
	if got := args.Get(0); got != nil {
		return got.(*domain.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) GetSessionByRefreshHash(ctx context.Context, refreshHash string) (*domain.Session, error) {
	args := m.Called(ctx, refreshHash)
	if got := args.Get(0); got != nil {
		return got.(*domain.Session), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAuthRepository) UpdateSessionLastUsed(ctx context.Context, sessionID domain.SessionID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAuthRepository) RevokeSession(ctx context.Context, sessionID domain.SessionID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

// AuthServiceTestSuite defines the test suite for AuthService
type AuthServiceTestSuite struct {
	suite.Suite
	authService domain.AuthService
	mockRepo    *MockAuthRepository
}

func (suite *AuthServiceTestSuite) SetupTest() {
	suite.mockRepo = new(MockAuthRepository)
	// Pass nil for user and wallet clients; not needed for these tests
	suite.authService = service.NewAuthService(
		suite.mockRepo,
		nil,
		nil,
		nil,
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-jwt-secret"),
		false, // enableCollectionContext
	)
}

func (suite *AuthServiceTestSuite) TestGetNonce_Success() {
	ctx := context.Background()
	accountID := "0x1234567890123456789012345678901234567890"
	chainID := "eip155:1"
	domainName := "localhost"

	suite.mockRepo.On("CreateNonce", ctx, mock.AnythingOfType("*domain.Nonce")).Return(nil)

	nonce, err := suite.authService.GetNonce(ctx, accountID, chainID, domainName)

	suite.NoError(err)
	suite.NotEmpty(nonce)
	suite.mockRepo.AssertExpectations(suite.T())
}

// Skipping VerifySiwe success due to signature verification dependency.

func (suite *AuthServiceTestSuite) TestRefreshSession_Success() {
	ctx := context.Background()
	// 64-char hex string to satisfy format validation
	refreshToken := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	mockSession := &domain.Session{
		ID:          domain.SessionID("550e8400-e29b-41d4-a716-446655440000"),
		UserID:      domain.UserID("user-123"),
		RefreshHash: refreshToken,
		CreatedAt:   time.Now().Add(-1 * time.Hour),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		RevokedAt:   nil,
		LastUsedAt:  nil,
	}

	suite.mockRepo.On("GetSessionByRefreshHash", ctx, mock.AnythingOfType("string")).Return(mockSession, nil)
	suite.mockRepo.On("UpdateSessionLastUsed", ctx, mock.Anything).Return(nil)

	result, err := suite.authService.Refresh(ctx, refreshToken)

	suite.NoError(err)
	suite.NotEmpty(result.AccessToken)
	suite.NotEmpty(result.RefreshToken)
	suite.Equal(domain.UserID("user-123"), result.UserID)
	suite.mockRepo.AssertExpectations(suite.T())
}

func (suite *AuthServiceTestSuite) TestRefreshSession_InvalidToken() {
	ctx := context.Background()
	refreshToken := "short"
	result, err := suite.authService.Refresh(ctx, refreshToken)

	suite.Error(err)
	suite.Nil(result)
}

func (suite *AuthServiceTestSuite) TestRevokeSession_Success() {
	ctx := context.Background()
	// Use a valid UUID as required by the service
	sessionID := "550e8400-e29b-41d4-a716-446655440000"

	suite.mockRepo.On("RevokeSession", ctx, domain.SessionID(sessionID)).Return(nil)

	err := suite.authService.Logout(ctx, sessionID)

	suite.NoError(err)
	suite.mockRepo.AssertExpectations(suite.T())
}

func TestAuthServiceTestSuite(t *testing.T) {
	suite.Run(t, new(AuthServiceTestSuite))
}
