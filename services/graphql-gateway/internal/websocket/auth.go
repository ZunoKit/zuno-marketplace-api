package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/quangdang46/NFT-Marketplace/services/graphql-gateway/internal/auth"
	authProto "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
)

// WebSocketAuth handles WebSocket authentication
type WebSocketAuth struct {
	authClient     authProto.AuthServiceClient
	authMiddleware *auth.Middleware
}

// NewWebSocketAuth creates a new WebSocket authentication handler
func NewWebSocketAuth(authClient authProto.AuthServiceClient, jwtSecret []byte) *WebSocketAuth {
	return &WebSocketAuth{
		authClient:     authClient,
		authMiddleware: auth.NewMiddleware(jwtSecret, authClient),
	}
}

// InitFunc handles WebSocket connection initialization
func (w *WebSocketAuth) InitFunc(ctx context.Context, initPayload transport.InitPayload) (context.Context, error) {
	// Convert InitPayload to map
	params := make(map[string]interface{})
	for k, v := range initPayload {
		params[k] = v
	}

	// Authenticate using middleware
	ctx, err := w.authMiddleware.WebSocketAuthMiddleware(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Check if user is authenticated
	userID := auth.GetUserIDFromContext(ctx)
	if userID == "" {
		// Try to authenticate with refresh token if available
		refreshToken := auth.GetRefreshTokenFromHeader(ctx)
		if refreshToken != "" {
			// Attempt to refresh session
			resp, err := w.authClient.RefreshSession(ctx, &authProto.RefreshSessionRequest{
				RefreshToken: refreshToken,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to refresh session: %w", err)
			}

			// Update context with user info
			ctx = auth.WithUserID(ctx, resp.UserId)

			// Note: In production, you'd want to send the new tokens back to client
			// This could be done via a subscription message
		} else {
			return nil, fmt.Errorf("authentication required")
		}
	}

	return ctx, nil
}

// PingPongInterval returns the interval for WebSocket ping/pong
func (w *WebSocketAuth) PingPongInterval() time.Duration {
	return 10 * time.Second
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// ConnectionUpdateMessage is sent to update connection authentication
type ConnectionUpdateMessage struct {
	Type  string `json:"type"`  // "connection_update"
	Token string `json:"token"` // New access token
}

// HandleConnectionUpdate handles connection update messages
func (w *WebSocketAuth) HandleConnectionUpdate(ctx context.Context, msg ConnectionUpdateMessage) (context.Context, error) {
	if msg.Token == "" {
		return ctx, fmt.Errorf("token required for connection update")
	}

	// Verify new token
	claims, err := w.authMiddleware.VerifyToken(msg.Token)
	if err != nil {
		return ctx, fmt.Errorf("invalid token: %w", err)
	}

	// Update context with new user info
	ctx = auth.WithUserID(ctx, claims.UserID)
	ctx = auth.WithSessionID(ctx, claims.SessionID)

	return ctx, nil
}

// WebSocketErrorCode represents WebSocket error codes
type WebSocketErrorCode int

const (
	// Standard WebSocket close codes
	CloseNormalClosure           WebSocketErrorCode = 1000
	CloseGoingAway               WebSocketErrorCode = 1001
	CloseProtocolError           WebSocketErrorCode = 1002
	CloseUnsupportedData         WebSocketErrorCode = 1003
	CloseNoStatusReceived        WebSocketErrorCode = 1005
	CloseAbnormalClosure         WebSocketErrorCode = 1006
	CloseInvalidFramePayloadData WebSocketErrorCode = 1007
	ClosePolicyViolation         WebSocketErrorCode = 1008
	CloseMessageTooBig           WebSocketErrorCode = 1009
	CloseMandatoryExtension      WebSocketErrorCode = 1010
	CloseInternalServerErr       WebSocketErrorCode = 1011
	CloseServiceRestart          WebSocketErrorCode = 1012
	CloseTryAgainLater           WebSocketErrorCode = 1013
	CloseTLSHandshake            WebSocketErrorCode = 1015

	// Custom authentication error codes
	CloseUnauthorized WebSocketErrorCode = 4401 // Authentication required
	CloseForbidden    WebSocketErrorCode = 4403 // Access denied
	CloseTokenExpired WebSocketErrorCode = 4401 // Token expired
)

// WebSocketError represents a WebSocket error with code
type WebSocketError struct {
	Code    WebSocketErrorCode
	Message string
}

func (e WebSocketError) Error() string {
	return fmt.Sprintf("websocket error %d: %s", e.Code, e.Message)
}

// NewAuthError creates an authentication error
func NewAuthError(message string) error {
	return WebSocketError{
		Code:    CloseUnauthorized,
		Message: message,
	}
}

// NewTokenExpiredError creates a token expired error
func NewTokenExpiredError() error {
	return WebSocketError{
		Code:    CloseTokenExpired,
		Message: "token expired",
	}
}

// AuthenticateWebSocket is a helper function to authenticate WebSocket connections
func AuthenticateWebSocket(ctx context.Context, token string, authClient authProto.AuthServiceClient, jwtSecret []byte) (context.Context, error) {
	if token == "" {
		return nil, NewAuthError("authentication required")
	}

	// Create middleware to verify token
	middleware := auth.NewMiddleware(jwtSecret, authClient)

	// Verify token
	claims, err := middleware.VerifyToken(token)
	if err != nil {
		if err.Error() == "token expired" {
			return nil, NewTokenExpiredError()
		}
		return nil, NewAuthError(fmt.Sprintf("invalid token: %v", err))
	}

	// Add user info to context
	ctx = auth.WithUserID(ctx, claims.UserID)
	ctx = auth.WithSessionID(ctx, claims.SessionID)

	return ctx, nil
}
