package test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/service"
	protoUser "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	protoWallet "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
)

// Mock repository for testing
type MockAuthRepository struct {
	mock.Mock
}

func (m *MockAuthRepository) CreateNonce(ctx context.Context, nonce *domain.Nonce) error {
	args := m.Called(ctx, nonce)
	return args.Error(0)
}

func (m *MockAuthRepository) GetNonce(ctx context.Context, value string) (*domain.Nonce, error) {
	args := m.Called(ctx, value)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Nonce), args.Error(1)
}

func (m *MockAuthRepository) TryUseNonce(ctx context.Context, value, accountID, chainID, domain string, usedAt time.Time) (bool, error) {
	args := m.Called(ctx, value, accountID, chainID, domain, usedAt)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	args := m.Called(ctx, session)
	return args.Error(0)
}

func (m *MockAuthRepository) GetSession(ctx context.Context, sessionID domain.SessionID) (*domain.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockAuthRepository) GetSessionByRefreshHash(ctx context.Context, refreshHash string) (*domain.Session, error) {
	args := m.Called(ctx, refreshHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Session), args.Error(1)
}

func (m *MockAuthRepository) UpdateSessionLastUsed(ctx context.Context, sessionID domain.SessionID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAuthRepository) RevokeSession(ctx context.Context, sessionID domain.SessionID) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockAuthRepository) RevokeSessionWithReason(ctx context.Context, sessionID domain.SessionID, reason string) error {
	args := m.Called(ctx, sessionID, reason)
	return args.Error(0)
}

func (m *MockAuthRepository) RotateRefreshToken(ctx context.Context, sessionID domain.SessionID, newRefreshHash string) error {
	args := m.Called(ctx, sessionID, newRefreshHash)
	return args.Error(0)
}

func (m *MockAuthRepository) GetSessionsByTokenFamily(ctx context.Context, tokenFamilyID string) ([]*domain.Session, error) {
	args := m.Called(ctx, tokenFamilyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Session), args.Error(1)
}

func (m *MockAuthRepository) RevokeTokenFamily(ctx context.Context, tokenFamilyID string, reason string) error {
	args := m.Called(ctx, tokenFamilyID, reason)
	return args.Error(0)
}

func (m *MockAuthRepository) CheckTokenReuse(ctx context.Context, refreshHash string) (bool, error) {
	args := m.Called(ctx, refreshHash)
	return args.Bool(0), args.Error(1)
}

// Test successful token rotation
func TestRefreshTokenRotation_Success(t *testing.T) {
	ctx := context.Background()

	// Setup
	mockRepo := new(MockAuthRepository)
	mockUserService := new(MockUserService)
	mockWalletService := new(MockWalletService)

	authService := service.NewAuthService(
		mockRepo,
		mockUserService,
		mockWalletService,
		nil, // publisher
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-secret"),
		false,
	)

	// Create test session
	sessionID := uuid.New().String()
	userID := uuid.New().String()
	tokenFamilyID := uuid.New().String()
	oldRefreshToken := "old-refresh-token-64-chars-0123456789abcdef0123456789abcdef01234567"
	oldRefreshHash := service.HashRefreshToken(oldRefreshToken)

	session := &domain.Session{
		ID:              domain.SessionID(sessionID),
		UserID:          domain.UserID(userID),
		RefreshHash:     oldRefreshHash,
		TokenFamilyID:   tokenFamilyID,
		TokenGeneration: 1,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
		CreatedAt:       time.Now(),
	}

	// Mock expectations
	mockRepo.On("CheckTokenReuse", ctx, oldRefreshHash).Return(false, nil)
	mockRepo.On("GetSessionByRefreshHash", ctx, oldRefreshHash).Return(session, nil)
	mockRepo.On("RotateRefreshToken", ctx, domain.SessionID(sessionID), mock.AnythingOfType("string")).Return(nil)
	mockRepo.On("UpdateSessionLastUsed", ctx, domain.SessionID(sessionID)).Return(nil)

	// Execute
	result, err := authService.Refresh(ctx, oldRefreshToken)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
	assert.NotEqual(t, oldRefreshToken, result.RefreshToken, "New refresh token should be different")
	assert.Equal(t, userID, string(result.UserID))

	mockRepo.AssertExpectations(t)
}

// Test replay attack detection
func TestRefreshTokenRotation_ReplayAttackDetection(t *testing.T) {
	ctx := context.Background()

	// Setup
	mockRepo := new(MockAuthRepository)
	mockUserService := new(MockUserService)
	mockWalletService := new(MockWalletService)

	authService := service.NewAuthService(
		mockRepo,
		mockUserService,
		mockWalletService,
		nil,
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-secret"),
		false,
	)

	// Create test session
	sessionID := uuid.New().String()
	userID := uuid.New().String()
	tokenFamilyID := uuid.New().String()
	reusedToken := "reused-refresh-token-64-chars-0123456789abcdef0123456789abcdef0123"
	reusedHash := service.HashRefreshToken(reusedToken)

	session := &domain.Session{
		ID:                  domain.SessionID(sessionID),
		UserID:              domain.UserID(userID),
		RefreshHash:         "current-hash",
		PreviousRefreshHash: &reusedHash, // This token was already used
		TokenFamilyID:       tokenFamilyID,
		TokenGeneration:     2,
		ExpiresAt:           time.Now().Add(24 * time.Hour),
		CreatedAt:           time.Now(),
	}

	// Mock expectations - token reuse detected
	mockRepo.On("CheckTokenReuse", ctx, reusedHash).Return(true, nil)
	mockRepo.On("GetSessionByRefreshHash", ctx, reusedHash).Return(session, nil)
	mockRepo.On("RevokeTokenFamily", ctx, tokenFamilyID, "replay_attack_detected").Return(nil)

	// Execute
	result, err := authService.Refresh(ctx, reusedToken)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refresh token has been compromised")
	assert.Nil(t, result)

	mockRepo.AssertExpectations(t)
}

// Test expired session
func TestRefreshTokenRotation_ExpiredSession(t *testing.T) {
	ctx := context.Background()

	// Setup
	mockRepo := new(MockAuthRepository)
	mockUserService := new(MockUserService)
	mockWalletService := new(MockWalletService)

	authService := service.NewAuthService(
		mockRepo,
		mockUserService,
		mockWalletService,
		nil,
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-secret"),
		false,
	)

	// Create expired session
	sessionID := uuid.New().String()
	userID := uuid.New().String()
	tokenFamilyID := uuid.New().String()
	refreshToken := "refresh-token-64-chars-0123456789abcdef0123456789abcdef01234567"
	refreshHash := service.HashRefreshToken(refreshToken)

	session := &domain.Session{
		ID:              domain.SessionID(sessionID),
		UserID:          domain.UserID(userID),
		RefreshHash:     refreshHash,
		TokenFamilyID:   tokenFamilyID,
		TokenGeneration: 1,
		ExpiresAt:       time.Now().Add(-1 * time.Hour), // Expired
		CreatedAt:       time.Now().Add(-25 * time.Hour),
	}

	// Mock expectations
	mockRepo.On("CheckTokenReuse", ctx, refreshHash).Return(false, nil)
	mockRepo.On("GetSessionByRefreshHash", ctx, refreshHash).Return(session, nil)

	// Execute
	result, err := authService.Refresh(ctx, refreshToken)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session has expired")
	assert.Nil(t, result)

	mockRepo.AssertExpectations(t)
}

// Test revoked session
func TestRefreshTokenRotation_RevokedSession(t *testing.T) {
	ctx := context.Background()

	// Setup
	mockRepo := new(MockAuthRepository)
	mockUserService := new(MockUserService)
	mockWalletService := new(MockWalletService)

	authService := service.NewAuthService(
		mockRepo,
		mockUserService,
		mockWalletService,
		nil,
		[]byte("test-jwt-secret"),
		[]byte("test-refresh-secret"),
		false,
	)

	// Create revoked session
	sessionID := uuid.New().String()
	userID := uuid.New().String()
	tokenFamilyID := uuid.New().String()
	refreshToken := "refresh-token-64-chars-0123456789abcdef0123456789abcdef01234567"
	refreshHash := service.HashRefreshToken(refreshToken)
	revokedAt := time.Now().Add(-1 * time.Hour)

	session := &domain.Session{
		ID:              domain.SessionID(sessionID),
		UserID:          domain.UserID(userID),
		RefreshHash:     refreshHash,
		TokenFamilyID:   tokenFamilyID,
		TokenGeneration: 1,
		ExpiresAt:       time.Now().Add(24 * time.Hour),
		CreatedAt:       time.Now().Add(-2 * time.Hour),
		RevokedAt:       &revokedAt, // Session is revoked
	}

	// Mock expectations
	mockRepo.On("CheckTokenReuse", ctx, refreshHash).Return(false, nil)
	mockRepo.On("GetSessionByRefreshHash", ctx, refreshHash).Return(session, nil)

	// Execute
	result, err := authService.Refresh(ctx, refreshToken)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "session has been revoked")
	assert.Nil(t, result)

	mockRepo.AssertExpectations(t)
}

// Mock services for testing
type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) CreateUser(ctx context.Context, in *protoUser.CreateUserRequest) (*protoUser.CreateUserResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*protoUser.CreateUserResponse), args.Error(1)
}

type MockWalletService struct {
	mock.Mock
}

func (m *MockWalletService) LinkWallet(ctx context.Context, in *protoWallet.LinkWalletRequest) (*protoWallet.LinkWalletResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*protoWallet.LinkWalletResponse), args.Error(1)
}
