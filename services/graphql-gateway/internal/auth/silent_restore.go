package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	authProto "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
)

// SilentRestoreService handles silent authentication restoration on app startup
type SilentRestoreService struct {
	authClient authProto.AuthServiceClient
	middleware *Middleware
	tokenStore TokenStore
}

// NewSilentRestoreService creates a new silent restore service
func NewSilentRestoreService(authClient authProto.AuthServiceClient, middleware *Middleware, tokenStore TokenStore) *SilentRestoreService {
	return &SilentRestoreService{
		authClient: authClient,
		middleware: middleware,
		tokenStore: tokenStore,
	}
}

// RestoreSession attempts to silently restore the user session on app startup
func (s *SilentRestoreService) RestoreSession(ctx context.Context, r *http.Request) (*AuthenticationResult, error) {
	// Try to get refresh token from cookie
	refreshToken := GetRefreshTokenFromCookie(r)
	if refreshToken == "" {
		// No refresh token, user is not authenticated
		return &AuthenticationResult{
			IsAuthenticated: false,
		}, nil
	}

	// Try to refresh the session
	resp, err := s.authClient.RefreshSession(ctx, &authProto.RefreshSessionRequest{
		RefreshToken: refreshToken,
		UserAgent:    r.UserAgent(),
		IpAddress:    GetClientIP(r),
	})
	if err != nil {
		// Refresh failed, user needs to re-authenticate
		return &AuthenticationResult{
			IsAuthenticated: false,
			Error:           fmt.Sprintf("session refresh failed: %v", err),
		}, nil
	}

	// Store new tokens
	s.tokenStore.SetTokens(resp.AccessToken, resp.RefreshToken)

	// Parse expiry time
	expiresAt, _ := time.Parse(time.RFC3339, resp.ExpiresAt)

	// Return successful authentication
	return &AuthenticationResult{
		IsAuthenticated: true,
		AccessToken:     resp.AccessToken,
		RefreshToken:    resp.RefreshToken,
		UserID:          resp.UserId,
		ExpiresAt:       expiresAt,
	}, nil
}

// TryRestoreSession is a middleware that attempts to restore session on each request
func (s *SilentRestoreService) TryRestoreSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if authorization header exists
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			// User already has auth header, proceed
			next.ServeHTTP(w, r)
			return
		}

		// Try to restore from refresh token cookie
		result, err := s.RestoreSession(r.Context(), r)
		if err != nil {
			// Log error but continue - this is silent restoration
			fmt.Printf("Silent restore error: %v\n", err)
		}

		if result != nil && result.IsAuthenticated {
			// Add access token to request header for downstream services
			r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", result.AccessToken))

			// Update refresh token cookie
			SetRefreshTokenCookie(w, result.RefreshToken)

			// Add user info to context
			ctx := WithUserID(r.Context(), result.UserID)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// InitializeAuth handles initial authentication check on app load
func (s *SilentRestoreService) InitializeAuth(ctx context.Context, r *http.Request, w http.ResponseWriter) (*InitAuthResponse, error) {
	// Check for existing authentication
	result, err := s.RestoreSession(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to restore session: %w", err)
	}

	response := &InitAuthResponse{
		IsAuthenticated: result.IsAuthenticated,
	}

	if result.IsAuthenticated {
		// Set cookies and return user info
		SetRefreshTokenCookie(w, result.RefreshToken)

		response.User = &UserInfo{
			ID:          result.UserID,
			AccessToken: result.AccessToken,
			ExpiresAt:   result.ExpiresAt,
		}

		// Fetch additional user data if needed
		if s.authClient != nil {
			// Could fetch user profile, wallets, etc.
			// This would be implementation-specific
		}
	}

	return response, nil
}

// AuthenticationResult represents the result of an authentication attempt
type AuthenticationResult struct {
	IsAuthenticated bool
	AccessToken     string
	RefreshToken    string
	UserID          string
	ExpiresAt       time.Time
	Error           string
}

// InitAuthResponse represents the initial authentication response
type InitAuthResponse struct {
	IsAuthenticated bool      `json:"isAuthenticated"`
	User            *UserInfo `json:"user,omitempty"`
	RequiresAuth    bool      `json:"requiresAuth"`
	AuthEndpoint    string    `json:"authEndpoint,omitempty"`
}

// UserInfo represents basic user information
type UserInfo struct {
	ID          string    `json:"id"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Wallets     []string  `json:"wallets,omitempty"`
}

// GetRefreshTokenFromCookie extracts refresh token from HTTP cookie
func GetRefreshTokenFromCookie(r *http.Request) string {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// SetRefreshTokenCookie sets the refresh token as an HttpOnly cookie
func SetRefreshTokenCookie(w http.ResponseWriter, refreshToken string) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Use HTTPS in production
		SameSite: http.SameSiteStrictMode,
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	}
	http.SetCookie(w, cookie)
}

// ClearRefreshTokenCookie clears the refresh token cookie
func ClearRefreshTokenCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1, // Delete cookie
	}
	http.SetCookie(w, cookie)
}

// GetClientIP extracts client IP address from request
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return forwarded
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}
