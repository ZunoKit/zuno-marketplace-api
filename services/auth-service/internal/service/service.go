package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/spruceid/siwe-go"
	"google.golang.org/grpc/metadata"

	"github.com/quangdang46/NFT-Marketplace/services/auth-service/internal/domain"

	protoUser "github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	protoWallet "github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"

)

type Service struct {
	authRepo      domain.AuthRepository
	userService   protoUser.UserServiceClient     // gRPC client to user service
	walletService protoWallet.WalletServiceClient // gRPC client to wallet service
	publisher     domain.AuthEventPublisher       // event publisher
	jwtSecret     []byte
	refreshSecret []byte
	nonceTTL      time.Duration
	sessionTTL    time.Duration
	enableCollectionContext bool
}

func NewAuthService(
	authRepo domain.AuthRepository,
	userService protoUser.UserServiceClient,
	walletService protoWallet.WalletServiceClient,
	publisher domain.AuthEventPublisher,
	jwtSecret, refreshSecret []byte,
	enableCollectionContext bool,
) domain.AuthService {
	return &Service{
		authRepo:      authRepo,
		userService:   userService,
		walletService: walletService,
		publisher:     publisher,
		jwtSecret:     jwtSecret,
		refreshSecret: refreshSecret,
		nonceTTL:      5 * time.Minute,
		sessionTTL:    24 * time.Hour,
		enableCollectionContext: enableCollectionContext,
	}
}

func (s *Service) GetNonce(ctx context.Context, accountID, chainID, domainName string) (string, error) {
	// Validate inputs
	if err := s.validateGetNonceInputs(accountID, chainID, domainName); err != nil {
		return "", err
	}

	// Generate cryptographically secure random nonce
	nonce, err := s.generateSecureNonce()
	if err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create nonce record
	now := time.Now()
	nonceRecord := &domain.Nonce{
		Value:     nonce,
		AccountID: strings.ToLower(accountID), // Normalize to lowercase
		ChainID:   domain.ChainID(chainID),
		Domain:    domainName,
		Used:      false,
		ExpiresAt: now.Add(s.nonceTTL),
		CreatedAt: now,
	}

	// Store nonce in repository
	if err := s.authRepo.CreateNonce(ctx, nonceRecord); err != nil {
		return "", fmt.Errorf("failed to create nonce: %w", err)
	}

	return nonce, nil
}

func (s *Service) VerifySiwe(ctx context.Context, accountID, message, signature string) (*domain.AuthResult, error) {
	// Parse SIWE message
	siweMessage, err := siwe.ParseMessage(message)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SIWE message: %w", err)
	}

	// Validate basic message fields
	if err := s.validateSiweMessage(siweMessage, accountID); err != nil {
		return nil, err
	}

	// Verify signature
	publicKey, err := siweMessage.VerifyEIP191(signature)
	if err != nil {
		return nil, fmt.Errorf("failed to verify signature: %w", err)
	}

	// Verify the recovered address matches the expected account
	recoveredAddress := crypto.PubkeyToAddress(*publicKey)
	expectedAddress := common.HexToAddress(accountID)
	if recoveredAddress != expectedAddress {
		return nil, fmt.Errorf("signature verification failed: address mismatch")
	}

	// Convert chain ID to string
	chainIDStr := fmt.Sprintf("eip155:%d", siweMessage.GetChainID())

	// Validate and consume nonce
	success, err := s.authRepo.TryUseNonce(ctx, siweMessage.GetNonce(), accountID, chainIDStr, siweMessage.GetDomain(), time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to validate nonce: %w", err)
	}
	if !success {
		return nil, fmt.Errorf("nonce validation failed: nonce may be expired, used, or invalid")
	}

	// Ensure user exists (create if needed)
	userResp, err := s.userService.EnsureUser(ctx, &protoUser.EnsureUserRequest{
		AccountId: accountID,
		Address:   strings.ToLower(accountID),
		ChainId:   chainIDStr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to ensure user: %w", err)
	}

	// Link wallet
	_, err = s.walletService.UpsertLink(ctx, &protoWallet.UpsertLinkRequest{
		UserId:    userResp.GetUserId(),
		AccountId: accountID,
		Address:   strings.ToLower(accountID),
		ChainId:   chainIDStr,
		IsPrimary: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to link wallet: %w", err)
	}

	// Create session
	sessionID := uuid.New().String()
	refreshToken := s.generateRefreshToken()
	refreshHash := s.hashRefreshToken(refreshToken)

	now := time.Now()
	session := &domain.Session{
		ID:          domain.SessionID(sessionID),
		UserID:      domain.UserID(userResp.GetUserId()),
		RefreshHash: refreshHash,
		ExpiresAt:   now.Add(s.sessionTTL),
		CreatedAt:   now,
		LastUsedAt:  &now,
	}

	// Optionally attach collection intent context when feature enabled and header present
	if s.enableCollectionContext {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			values := md.Get("x-prepare-collection")
			flag := len(values) > 0 && strings.EqualFold(values[0], "true")
			if flag {
				correlationID := uuid.New().String()
				ctxObj := map[string]any{
					"prepareCollection": true,
					"requestedAt":      now.UTC().Format(time.RFC3339Nano),
					"correlationId":    correlationID,
				}
				b, _ := json.Marshal(ctxObj)
				jsonStr := string(b)
				session.CollectionIntentContext = &jsonStr
			}
		}
	}

	if err := s.authRepo.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Audit: session create
	if session.CollectionIntentContext != nil {
		log.Printf("audit|event=session_create|session_id=%s|user_id=%s|collection_context=%s|timestamp=%s",
			sessionID, userResp.GetUserId(), *session.CollectionIntentContext, now.UTC().Format(time.RFC3339Nano))
	} else {
		log.Printf("audit|event=session_create|session_id=%s|user_id=%s|timestamp=%s",
			sessionID, userResp.GetUserId(), now.UTC().Format(time.RFC3339Nano))
	}

	// Generate JWT access token
	accessToken, expiresAt, err := s.generateAccessToken(userResp.GetUserId(), sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Publish auth.user_logged_in event (non-blocking)
	if s.publisher != nil {
		go func() {
			_ = s.publisher.PublishUserLoggedIn(context.Background(), &domain.AuthUserLoggedInEvent{
				UserID:     domain.UserID(userResp.GetUserId()),
				AccountID:  accountID,
				Address:    domain.Address(strings.ToLower(accountID)),
				ChainID:    domain.ChainID(chainIDStr),
				SessionID:  domain.SessionID(sessionID),
				LoggedInAt: now,
			})
		}()
	}

	return &domain.AuthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		UserID:       domain.UserID(userResp.GetUserId()),
		Address:      domain.Address(strings.ToLower(accountID)),
		ChainID:      domain.ChainID(chainIDStr),
	}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*domain.AuthResult, error) {
	// Validate refresh token format
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token is required")
	}

	// Validate refresh token format (should be 64 hex characters)
	if len(refreshToken) != 64 {
		return nil, fmt.Errorf("invalid refresh token format")
	}

	// Hash the refresh token to find the session
	refreshHash := s.hashRefreshToken(refreshToken)

	// Find active session by refresh token hash
	session, err := s.authRepo.GetSessionByRefreshHash(ctx, refreshHash)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired refresh token: %w", err)
	}

	// Double-check session hasn't expired (additional safety check)
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session has expired")
	}

	// Update session last used timestamp
	if err := s.authRepo.UpdateSessionLastUsed(ctx, session.ID); err != nil {
		// Log warning but don't fail the refresh operation
		// In production, you might want to log this error
	}

	// Generate new access token
	accessToken, expiresAt, err := s.generateAccessToken(string(session.UserID), string(session.ID))
	if err != nil {
		return nil, fmt.Errorf("failed to generate new access token: %w", err)
	}

	// For security, we could optionally generate a new refresh token
	// For now, we'll reuse the existing one to keep the session alive
	// In high-security environments, you might want to rotate refresh tokens

	// Optionally fetch user's primary wallet address and chain ID
	// For performance, we'll leave these empty in the refresh response
	// The client can use the user ID to fetch this information if needed

	// Audit: session refresh
	log.Printf("audit|event=session_refresh|session_id=%s|user_id=%s|timestamp=%s",
		string(session.ID), string(session.UserID), time.Now().UTC().Format(time.RFC3339Nano))

	return &domain.AuthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken, // Reusing existing refresh token
		ExpiresAt:    expiresAt,
		UserID:       session.UserID,
		Address:      "", // Not included in refresh response for performance
		ChainID:      "", // Not included in refresh response for performance
	}, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) error {
	// Validate session ID format
	if sessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	// Validate UUID format
	if _, err := uuid.Parse(sessionID); err != nil {
		return fmt.Errorf("invalid session ID format: %w", err)
	}

	// Revoke the session
	if err := s.authRepo.RevokeSession(ctx, domain.SessionID(sessionID)); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	return nil
}

func (s *Service) LogoutByRefreshToken(ctx context.Context, refreshToken string) error {
	// Validate refresh token format
	if refreshToken == "" {
		return fmt.Errorf("refresh token is required")
	}

	if len(refreshToken) != 64 {
		return fmt.Errorf("invalid refresh token format")
	}

	// Hash the refresh token to find the session
	refreshHash := s.hashRefreshToken(refreshToken)

	// Find active session by refresh token hash
	session, err := s.authRepo.GetSessionByRefreshHash(ctx, refreshHash)
	if err != nil {
		return fmt.Errorf("invalid or expired refresh token: %w", err)
	}

	// Revoke the session
	if err := s.authRepo.RevokeSession(ctx, session.ID); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	return nil
}

// validateGetNonceInputs validates the input parameters for GetNonce
func (s *Service) validateGetNonceInputs(accountID, chainID, domainName string) error {
	if accountID == "" {
		return domain.ErrInvalidAccountID
	}

	if chainID == "" {
		return domain.ErrInvalidChainID
	}

	if domainName == "" {
		return fmt.Errorf("domain is required")
	}

	// Validate Ethereum address format (0x followed by 40 hex characters)
	accountID = strings.ToLower(accountID)
	if !isValidEthereumAddress(accountID) {
		return domain.ErrInvalidAccountID
	}

	// Validate CAIP-2 chain ID format (e.g., "eip155:1")
	if !isValidCAIP2ChainID(chainID) {
		return domain.ErrInvalidChainID
	}

	return nil
}

// generateSecureNonce generates a cryptographically secure random nonce
func (s *Service) generateSecureNonce() (string, error) {
	// Generate 32 bytes of random data (256 bits)
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex string (64 characters)
	return hex.EncodeToString(bytes), nil
}

// isValidEthereumAddress validates Ethereum address format
func isValidEthereumAddress(address string) bool {
	// Must be lowercase 0x followed by exactly 40 hex characters
	matched, _ := regexp.MatchString(`^0x[0-9a-f]{40}$`, address)
	return matched
}

// isValidCAIP2ChainID validates CAIP-2 chain ID format
func isValidCAIP2ChainID(chainID string) bool {
	// CAIP-2 format: namespace:reference (e.g., "eip155:1", "eip155:137")
	matched, _ := regexp.MatchString(`^[a-z0-9]+:[a-zA-Z0-9]+$`, chainID)
	return matched
}

// validateSiweMessage validates SIWE message fields
func (s *Service) validateSiweMessage(message *siwe.Message, expectedAccountID string) error {
	// Validate address matches
	if !strings.EqualFold(message.GetAddress().Hex(), expectedAccountID) {
		return fmt.Errorf("address mismatch in SIWE message")
	}

	// Validate message is not expired
	if message.GetExpirationTime() != nil {
		expTime, err := time.Parse(time.RFC3339, *message.GetExpirationTime())
		if err != nil {
			return fmt.Errorf("invalid expiration time format: %w", err)
		}
		if time.Now().After(expTime) {
			return fmt.Errorf("SIWE message has expired")
		}
	}

	// Validate message is not used before valid time
	if message.GetNotBefore() != nil {
		notBeforeTime, err := time.Parse(time.RFC3339, *message.GetNotBefore())
		if err != nil {
			return fmt.Errorf("invalid not-before time format: %w", err)
		}
		if time.Now().Before(notBeforeTime) {
			return fmt.Errorf("SIWE message is not yet valid")
		}
	}

	// Validate nonce is present
	if message.GetNonce() == "" {
		return fmt.Errorf("nonce is required in SIWE message")
	}

	return nil
}

// generateRefreshToken generates a cryptographically secure refresh token
func (s *Service) generateRefreshToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// hashRefreshToken creates a hash of the refresh token for storage
func (s *Service) hashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// generateAccessToken creates a JWT access token
func (s *Service) generateAccessToken(userID, sessionID string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(1 * time.Hour) // Access tokens expire in 1 hour

	claims := jwt.MapClaims{
		"sub":        userID,
		"session_id": sessionID,
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"iss":        "nft-marketplace-auth",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to sign JWT token: %w", err)
	}

	return tokenString, expiresAt, nil
}
