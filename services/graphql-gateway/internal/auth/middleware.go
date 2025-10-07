package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const (
	UserIDKey       contextKey = "user_id"
	SessionIDKey    contextKey = "session_id"
	IPAddressKey    contextKey = "ip_address"
	RefreshTokenKey contextKey = "refresh_token"
)

// Middleware provides authentication middleware for GraphQL
type Middleware struct {
	jwtSecret  []byte
	authClient interface{} // Auth service client
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(jwtSecret []byte, authClient interface{}) *Middleware {
	return &Middleware{
		jwtSecret:  jwtSecret,
		authClient: authClient,
	}
}

// AuthMiddleware is the HTTP middleware for authentication
func (m *Middleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract access token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			token := extractBearerToken(authHeader)
			if token != "" {
				// Verify and parse JWT token
				claims, err := m.VerifyToken(token)
				if err == nil {
					// Add user info to context
					ctx = WithUserID(ctx, claims.UserID)
					ctx = WithSessionID(ctx, claims.SessionID)
				}
			}
		}

		// Extract refresh token from cookie
		cookie, err := r.Cookie("refresh_token")
		if err == nil && cookie.Value != "" {
			ctx = WithRefreshToken(ctx, cookie.Value)
		}

		// Add IP address to context
		ip := getClientIP(r)
		ctx = WithIPAddress(ctx, ip)

		// Continue with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// WebSocketAuthMiddleware handles WebSocket authentication
func (m *Middleware) WebSocketAuthMiddleware(ctx context.Context, payload map[string]interface{}) (context.Context, error) {
	// Extract token from connection params
	if authToken, ok := payload["Authorization"].(string); ok {
		token := extractBearerToken(authToken)
		if token != "" {
			claims, err := m.VerifyToken(token)
			if err != nil {
				return ctx, fmt.Errorf("invalid token: %w", err)
			}

			// Add user info to context
			ctx = WithUserID(ctx, claims.UserID)
			ctx = WithSessionID(ctx, claims.SessionID)
			return ctx, nil
		}
	}

	// Check for refresh token
	if refreshToken, ok := payload["refresh_token"].(string); ok {
		ctx = WithRefreshToken(ctx, refreshToken)
		// Note: Actual refresh should be handled by resolver
	}

	return ctx, nil
}

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID    string `json:"sub"`
	SessionID string `json:"session_id"`
	jwt.RegisteredClaims
}

// VerifyToken verifies and parses a JWT token
func (m *Middleware) VerifyToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		// Check expiration
		if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
			return nil, fmt.Errorf("token expired")
		}
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// extractBearerToken extracts token from Bearer authorization header
func extractBearerToken(authHeader string) string {
	parts := strings.Split(authHeader, " ")
	if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
		return parts[1]
	}
	return ""
}

// getClientIP gets the client IP address from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	return ip
}

// Context helper functions

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// GetUserIDFromContext gets user ID from context
func GetUserIDFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// WithSessionID adds session ID to context
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}

// GetSessionIDFromContext gets session ID from context
func GetSessionIDFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value(SessionIDKey).(string); ok {
		return sessionID
	}
	return ""
}

// WithIPAddress adds IP address to context
func WithIPAddress(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, IPAddressKey, ip)
}

// GetIPFromContext gets IP address from context
func GetIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(IPAddressKey).(string); ok {
		return ip
	}
	return ""
}

// WithRefreshToken adds refresh token to context
func WithRefreshToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, RefreshTokenKey, token)
}

// GetRefreshTokenFromHeader gets refresh token from context
func GetRefreshTokenFromHeader(ctx context.Context) string {
	if token, ok := ctx.Value(RefreshTokenKey).(string); ok {
		return token
	}
	return ""
}
