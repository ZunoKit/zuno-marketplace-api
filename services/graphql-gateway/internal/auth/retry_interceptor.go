package auth

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authProto "github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
)

// RetryInterceptor handles automatic token refresh on 401 errors
type RetryInterceptor struct {
	authClient   authProto.AuthServiceClient
	tokenStore   TokenStore
	maxRetries   int
	refreshMutex sync.Mutex
}

// TokenStore manages access and refresh tokens
type TokenStore interface {
	GetAccessToken() string
	GetRefreshToken() string
	SetTokens(accessToken, refreshToken string)
	ClearTokens()
}

// InMemoryTokenStore is a simple in-memory token store
type InMemoryTokenStore struct {
	mu           sync.RWMutex
	accessToken  string
	refreshToken string
}

func NewInMemoryTokenStore() *InMemoryTokenStore {
	return &InMemoryTokenStore{}
}

func (s *InMemoryTokenStore) GetAccessToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.accessToken
}

func (s *InMemoryTokenStore) GetRefreshToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.refreshToken
}

func (s *InMemoryTokenStore) SetTokens(accessToken, refreshToken string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = accessToken
	s.refreshToken = refreshToken
}

func (s *InMemoryTokenStore) ClearTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accessToken = ""
	s.refreshToken = ""
}

// NewRetryInterceptor creates a new retry interceptor
func NewRetryInterceptor(authClient authProto.AuthServiceClient, tokenStore TokenStore) *RetryInterceptor {
	return &RetryInterceptor{
		authClient: authClient,
		tokenStore: tokenStore,
		maxRetries: 1, // Only retry once
	}
}

// UnaryClientInterceptor is the gRPC unary interceptor for automatic retry
func (r *RetryInterceptor) UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Add access token to metadata if available
		ctx = r.addAuthMetadata(ctx)

		// First attempt
		err := invoker(ctx, method, req, reply, cc, opts...)

		// Check if we got 401 Unauthenticated
		if r.shouldRetry(err) {
			// Try to refresh token
			if r.refreshToken(ctx) {
				// Retry with new token
				ctx = r.addAuthMetadata(ctx)
				err = invoker(ctx, method, req, reply, cc, opts...)
			}
		}

		return err
	}
}

// StreamClientInterceptor is the gRPC stream interceptor for automatic retry
func (r *RetryInterceptor) StreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// Add access token to metadata
		ctx = r.addAuthMetadata(ctx)

		// First attempt
		stream, err := streamer(ctx, desc, cc, method, opts...)

		// Check if we got 401 Unauthenticated
		if r.shouldRetry(err) {
			// Try to refresh token
			if r.refreshToken(ctx) {
				// Retry with new token
				ctx = r.addAuthMetadata(ctx)
				stream, err = streamer(ctx, desc, cc, method, opts...)
			}
		}

		return stream, err
	}
}

// addAuthMetadata adds the access token to gRPC metadata
func (r *RetryInterceptor) addAuthMetadata(ctx context.Context) context.Context {
	token := r.tokenStore.GetAccessToken()
	if token != "" {
		md := metadata.New(map[string]string{
			"authorization": fmt.Sprintf("Bearer %s", token),
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}
	return ctx
}

// shouldRetry checks if the error is a 401 Unauthenticated
func (r *RetryInterceptor) shouldRetry(err error) bool {
	if err == nil {
		return false
	}

	st, ok := status.FromError(err)
	if !ok {
		return false
	}

	return st.Code() == codes.Unauthenticated
}

// refreshToken attempts to refresh the access token
func (r *RetryInterceptor) refreshToken(ctx context.Context) bool {
	r.refreshMutex.Lock()
	defer r.refreshMutex.Unlock()

	// Get current refresh token
	refreshToken := r.tokenStore.GetRefreshToken()
	if refreshToken == "" {
		return false
	}

	// Call auth service to refresh
	resp, err := r.authClient.RefreshSession(ctx, &authProto.RefreshSessionRequest{
		RefreshToken: refreshToken,
	})
	if err != nil {
		// Refresh failed, clear tokens
		r.tokenStore.ClearTokens()
		return false
	}

	// Update tokens
	r.tokenStore.SetTokens(resp.AccessToken, resp.RefreshToken)
	return true
}

// RequestAwareTokenStore stores tokens per request context
type RequestAwareTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*InMemoryTokenStore // Map request ID to token store
}

func NewRequestAwareTokenStore() *RequestAwareTokenStore {
	return &RequestAwareTokenStore{
		tokens: make(map[string]*InMemoryTokenStore),
	}
}

func (s *RequestAwareTokenStore) GetStore(requestID string) TokenStore {
	s.mu.Lock()
	defer s.mu.Unlock()

	if store, exists := s.tokens[requestID]; exists {
		return store
	}

	// Create new store for this request
	store := NewInMemoryTokenStore()
	s.tokens[requestID] = store
	return store
}

func (s *RequestAwareTokenStore) RemoveStore(requestID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, requestID)
}
