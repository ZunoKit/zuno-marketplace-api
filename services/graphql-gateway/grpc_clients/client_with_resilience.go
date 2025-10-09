package grpcclients

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/quangdang46/NFT-Marketplace/shared/proto/auth"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/user"
	"github.com/quangdang46/NFT-Marketplace/shared/proto/wallet"
	"github.com/quangdang46/NFT-Marketplace/shared/resilience"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ResilientAuthClient wraps AuthClient with circuit breaker
type ResilientAuthClient struct {
	*AuthClient
	circuitBreaker *resilience.CircuitBreaker
}

// NewResilientAuthClient creates a new auth client with circuit breaker
func NewResilientAuthClient(url string) *ResilientAuthClient {
	// Create base client
	baseClient := NewAuthClient(url)

	// Configure circuit breaker
	cbConfig := &resilience.CircuitBreakerConfig{
		Name:             "auth-service",
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenMaxCalls: 3,
		OnStateChange: func(name string, from, to resilience.State) {
			log.Printf("Circuit breaker '%s' state changed from %s to %s", name, from, to)
		},
	}

	return &ResilientAuthClient{
		AuthClient:     baseClient,
		circuitBreaker: resilience.NewCircuitBreaker(cbConfig),
	}
}

// GetNonce with circuit breaker
func (c *ResilientAuthClient) GetNonce(ctx context.Context, req *auth.GetNonceRequest) (*auth.GetNonceResponse, error) {
	var response *auth.GetNonceResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = (*c.Client).GetNonce(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// VerifySiwe with circuit breaker
func (c *ResilientAuthClient) VerifySiwe(ctx context.Context, req *auth.VerifySiweRequest) (*auth.VerifySiweResponse, error) {
	var response *auth.VerifySiweResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = (*c.Client).VerifySiwe(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// RefreshSession with circuit breaker
func (c *ResilientAuthClient) RefreshSession(ctx context.Context, req *auth.RefreshRequest) (*auth.RefreshResponse, error) {
	var response *auth.RefreshResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = (*c.Client).RefreshSession(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// Logout with circuit breaker
func (c *ResilientAuthClient) Logout(ctx context.Context, req *auth.LogoutRequest) (*auth.LogoutResponse, error) {
	var response *auth.LogoutResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = (*c.Client).Logout(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// ===========================================
// Resilient User Client
// ===========================================

// ResilientUserClient wraps UserClient with circuit breaker
type ResilientUserClient struct {
	client         user.UserServiceClient
	conn           *grpc.ClientConn
	circuitBreaker *resilience.CircuitBreaker
}

// NewResilientUserClient creates a new user client with circuit breaker
func NewResilientUserClient(url string) *ResilientUserClient {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		log.Fatalf("failed to dial user service: %v", err)
	}

	client := user.NewUserServiceClient(conn)

	// Configure circuit breaker
	cbConfig := &resilience.CircuitBreakerConfig{
		Name:             "user-service",
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenMaxCalls: 3,
		OnStateChange: func(name string, from, to resilience.State) {
			log.Printf("Circuit breaker '%s' state changed from %s to %s", name, from, to)
		},
	}

	return &ResilientUserClient{
		client:         client,
		conn:           conn,
		circuitBreaker: resilience.NewCircuitBreaker(cbConfig),
	}
}

// GetUser with circuit breaker
func (c *ResilientUserClient) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.GetUserResponse, error) {
	var response *user.GetUserResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = c.client.GetUser(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// CreateUser with circuit breaker
func (c *ResilientUserClient) CreateUser(ctx context.Context, req *user.CreateUserRequest) (*user.CreateUserResponse, error) {
	var response *user.CreateUserResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = c.client.CreateUser(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// UpdateUser with circuit breaker
func (c *ResilientUserClient) UpdateUser(ctx context.Context, req *user.UpdateUserRequest) (*user.UpdateUserResponse, error) {
	var response *user.UpdateUserResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = c.client.UpdateUser(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// ===========================================
// Resilient Wallet Client
// ===========================================

// ResilientWalletClient wraps WalletClient with circuit breaker
type ResilientWalletClient struct {
	client         wallet.WalletServiceClient
	conn           *grpc.ClientConn
	circuitBreaker *resilience.CircuitBreaker
}

// NewResilientWalletClient creates a new wallet client with circuit breaker
func NewResilientWalletClient(url string) *ResilientWalletClient {
	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	conn, err := grpc.Dial(url, dialOptions...)
	if err != nil {
		log.Fatalf("failed to dial wallet service: %v", err)
	}

	client := wallet.NewWalletServiceClient(conn)

	// Configure circuit breaker
	cbConfig := &resilience.CircuitBreakerConfig{
		Name:             "wallet-service",
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		HalfOpenMaxCalls: 3,
		OnStateChange: func(name string, from, to resilience.State) {
			log.Printf("Circuit breaker '%s' state changed from %s to %s", name, from, to)
		},
	}

	return &ResilientWalletClient{
		client:         client,
		conn:           conn,
		circuitBreaker: resilience.NewCircuitBreaker(cbConfig),
	}
}

// LinkWallet with circuit breaker
func (c *ResilientWalletClient) LinkWallet(ctx context.Context, req *wallet.LinkWalletRequest) (*wallet.LinkWalletResponse, error) {
	var response *wallet.LinkWalletResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = c.client.LinkWallet(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// UnlinkWallet with circuit breaker
func (c *ResilientWalletClient) UnlinkWallet(ctx context.Context, req *wallet.UnlinkWalletRequest) (*wallet.UnlinkWalletResponse, error) {
	var response *wallet.UnlinkWalletResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = c.client.UnlinkWallet(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// GetWallets with circuit breaker
func (c *ResilientWalletClient) GetWallets(ctx context.Context, req *wallet.GetWalletsRequest) (*wallet.GetWalletsResponse, error) {
	var response *wallet.GetWalletsResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = c.client.GetWallets(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// SetPrimaryWallet with circuit breaker
func (c *ResilientWalletClient) SetPrimaryWallet(ctx context.Context, req *wallet.SetPrimaryWalletRequest) (*wallet.SetPrimaryWalletResponse, error) {
	var response *wallet.SetPrimaryWalletResponse
	var callErr error

	err := c.circuitBreaker.Execute(ctx, func(ctx context.Context) error {
		response, callErr = c.client.SetPrimaryWallet(ctx, req)
		return callErr
	})

	if err != nil {
		return nil, fmt.Errorf("circuit breaker error: %w", err)
	}

	return response, nil
}

// Close closes the connection
func (c *ResilientAuthClient) Close() error {
	return c.conn.Close()
}

func (c *ResilientUserClient) Close() error {
	return c.conn.Close()
}

func (c *ResilientWalletClient) Close() error {
	return c.conn.Close()
}
