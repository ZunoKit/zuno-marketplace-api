package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/quangdang46/NFT-Marketplace/shared/env"
)

// AuthContextKey type for auth context keys
type AuthContextKey string

const (
	// CurrentUserKey is the context key for current user info
	CurrentUserKey AuthContextKey = "current_user"
	// SessionIDKey is the context key for session ID
	SessionIDKey AuthContextKey = "session_id"
)

// CurrentUser represents the authenticated user
type CurrentUser struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

// AuthMiddleware validates JWT Bearer tokens and adds user info to context
func AuthMiddleware(jwtSecret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract Bearer token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				// Parse Bearer token
				if strings.HasPrefix(authHeader, "Bearer ") {
					tokenString := strings.TrimPrefix(authHeader, "Bearer ")

					// Validate JWT token
					if user, err := validateJWTToken(tokenString, jwtSecret); err == nil {
						// Add user info to context
						ctx := context.WithValue(r.Context(), CurrentUserKey, user)
						ctx = context.WithValue(ctx, SessionIDKey, user.SessionID)
						r = r.WithContext(ctx)
					}
					// If token is invalid, we continue without setting user context
					// This allows both authenticated and unauthenticated requests
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetCurrentUser retrieves current user from context
func GetCurrentUser(ctx context.Context) *CurrentUser {
	if user, ok := ctx.Value(CurrentUserKey).(*CurrentUser); ok {
		return user
	}
	return nil
}

// GetSessionID retrieves session ID from context
func GetSessionID(ctx context.Context) string {
	if sessionID, ok := ctx.Value(SessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}

// RequireAuth returns error if user is not authenticated
func RequireAuth(ctx context.Context) (*CurrentUser, error) {
	user := GetCurrentUser(ctx)
	if user == nil {
		return nil, fmt.Errorf("authentication required")
	}
	return user, nil
}

// validateJWTToken validates JWT token and returns user info
func validateJWTToken(tokenString string, jwtSecret []byte) (*CurrentUser, error) {
	// Parse JWT token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Validate token and extract claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Extract user ID
		userID, ok := claims["sub"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid user ID in token")
		}

		// Extract session ID
		sessionID, ok := claims["session_id"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid session ID in token")
		}

		// Validate issuer
		if iss, ok := claims["iss"].(string); !ok || iss != "nft-marketplace-auth" {
			return nil, fmt.Errorf("invalid token issuer")
		}

		return &CurrentUser{
			UserID:    userID,
			SessionID: sessionID,
		}, nil
	}

	return nil, fmt.Errorf("invalid token claims")
}

// CreateAuthMiddleware creates auth middleware with configuration
func CreateAuthMiddleware() func(http.Handler) http.Handler {
	// Load JWT secret from environment
	jwtSecret := env.GetString("JWT_SECRET", "default-jwt-secret-for-development")

	return AuthMiddleware([]byte(jwtSecret))
}
