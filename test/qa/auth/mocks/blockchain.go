// Package mocks provides mock implementations of blockchain
// clients and related infrastructure for testing without
// requiring real blockchain connections.
package mocks

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/types"
)

// BlockchainClient is a mock blockchain client for testing
type BlockchainClient struct {
	mu                sync.RWMutex
	endpoints         []types.RPCEndpoint
	activeEndpoint    string
	failedEndpoints   map[string]bool
	slowResponses     map[string]time.Duration
	failureCount      int
	maxFailures       int
	usingMockBackend  bool
	testnetOnlyMode   bool
	onRetryCallback   func(int)
	retryAttempts     int
}

// NewBlockchainClient creates a new mock blockchain client
func NewBlockchainClient(endpoints []types.RPCEndpoint) *BlockchainClient {
	return &BlockchainClient{
		endpoints:        endpoints,
		failedEndpoints:  make(map[string]bool),
		slowResponses:    make(map[string]time.Duration),
		testnetOnlyMode:  true, // Always true for QA environment
		activeEndpoint:   endpoints[0].URL,
	}
}

// Connect attempts to connect to an endpoint
func (c *BlockchainClient) Connect(endpoint string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if mainnet connection is attempted in testnet-only mode
	if c.testnetOnlyMode {
		endpointLower := strings.ToLower(endpoint)
		mainnetPatterns := []string{
			"mainnet.infura.io",
			"eth-mainnet",
			"cloudflare-eth.com",
		}

		for _, pattern := range mainnetPatterns {
			if strings.Contains(endpointLower, pattern) {
				return fmt.Errorf("mainnet connections not allowed in test mode")
			}
		}
	}

	// Check if endpoint is marked as failed
	if c.failedEndpoints[endpoint] {
		return fmt.Errorf("endpoint %s is unavailable", endpoint)
	}

	// Simulate slow response if configured
	if delay, exists := c.slowResponses[endpoint]; exists {
		time.Sleep(delay)
	}

	c.activeEndpoint = endpoint
	return nil
}

// GetChainID returns the chain ID (Sepolia for tests)
func (c *BlockchainClient) GetChainID(ctx context.Context) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Track retry attempts (any call after the first is a retry)
	if c.onRetryCallback != nil && c.failureCount > 0 {
		c.onRetryCallback(c.retryAttempts)
		c.retryAttempts++
	}

	// Simulate failures if configured
	if c.failureCount < c.maxFailures {
		c.failureCount++
		return 0, fmt.Errorf("network failure simulation")
	}

	// Check for slow response
	if delay, exists := c.slowResponses[c.activeEndpoint]; exists {
		c.mu.Unlock() // Unlock during sleep
		select {
		case <-time.After(delay):
			c.mu.Lock()
			return 0, fmt.Errorf("timeout: request exceeded deadline")
		case <-ctx.Done():
			c.mu.Lock()
			// Return timeout error even if context is cancelled
			return 0, fmt.Errorf("timeout: %w", ctx.Err())
		}
	}

	// Try active endpoint first
	if !c.failedEndpoints[c.activeEndpoint] {
		return 11155111, nil // Sepolia chain ID
	}

	// Try failover to other endpoints
	for _, endpoint := range c.endpoints {
		if !c.failedEndpoints[endpoint.URL] {
			c.activeEndpoint = endpoint.URL
			return 11155111, nil
		}
	}

	// If all endpoints failed and mock fallback is enabled
	if c.usingMockBackend {
		return 11155111, nil
	}

	return 0, fmt.Errorf("all endpoints unavailable")
}

// GetActiveEndpoint returns the currently active endpoint
func (c *BlockchainClient) GetActiveEndpoint() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeEndpoint
}

// IsUsingMockBackend returns whether mock backend is active
func (c *BlockchainClient) IsUsingMockBackend() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.usingMockBackend
}

// SimulateEndpointFailure marks an endpoint as failed
func (c *BlockchainClient) SimulateEndpointFailure(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failedEndpoints[endpoint] = true

	// If all endpoints failed, enable mock backend
	allFailed := true
	for _, ep := range c.endpoints {
		if !c.failedEndpoints[ep.URL] {
			allFailed = false
			break
		}
	}
	if allFailed {
		c.usingMockBackend = true
	}
}

// SimulateSlowResponse configures slow response for an endpoint
func (c *BlockchainClient) SimulateSlowResponse(delay time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.slowResponses[c.activeEndpoint] = delay
}

// SimulateFailures configures number of failures before success
func (c *BlockchainClient) SimulateFailures(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxFailures = count
	c.failureCount = 0
}

// OnRetry sets a callback for retry events
func (c *BlockchainClient) OnRetry(callback func(int)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onRetryCallback = callback
	c.retryAttempts = 0
}

// NewBlockchainClientWithFailover creates a client with failover support
func NewBlockchainClientWithFailover(endpoints []types.RPCEndpoint) *BlockchainClient {
	return NewBlockchainClient(endpoints)
}

// NewBlockchainClientWithRetry creates a client with retry configuration
func NewBlockchainClientWithRetry(maxRetries int, initialBackoff time.Duration) *BlockchainClient {
	client := &BlockchainClient{
		endpoints:       []types.RPCEndpoint{{URL: "https://rpc.sepolia.org", Priority: 1}},
		failedEndpoints: make(map[string]bool),
		slowResponses:   make(map[string]time.Duration),
		testnetOnlyMode: true,
	}
	return client
}

// NewBlockchainClientWithMockFallback creates a client with mock fallback
func NewBlockchainClientWithMockFallback(config *types.TestnetConfig) *BlockchainClient {
	client := NewBlockchainClient(config.Sepolia.RPCEndpoints)
	// Mock fallback enabled by default when all endpoints fail
	return client
}

// NewBlockchainClientWithTimeout creates a client with timeout configuration
func NewBlockchainClientWithTimeout(timeout time.Duration) *BlockchainClient {
	client := &BlockchainClient{
		endpoints:       []types.RPCEndpoint{{URL: "https://rpc.sepolia.org", Priority: 1}},
		failedEndpoints: make(map[string]bool),
		slowResponses:   make(map[string]time.Duration),
		testnetOnlyMode: true,
	}
	return client
}

// NetworkHealthMonitor monitors network health
type NetworkHealthMonitor struct {
	endpoints []types.RPCEndpoint
	health    *NetworkHealth
	mu        sync.RWMutex
}

// NetworkHealth represents network health status
type NetworkHealth struct {
	Endpoints []EndpointHealth
}

// EndpointHealth represents individual endpoint health
type EndpointHealth struct {
	URL    string
	Status string
}

// NewNetworkHealthMonitor creates a new network health monitor
func NewNetworkHealthMonitor(endpoints []types.RPCEndpoint) *NetworkHealthMonitor {
	return &NetworkHealthMonitor{
		endpoints: endpoints,
		health: &NetworkHealth{
			Endpoints: make([]EndpointHealth, 0),
		},
	}
}

// Start begins health monitoring
func (m *NetworkHealthMonitor) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkHealth()
			}
		}
	}()
}

// checkHealth checks endpoint health
func (m *NetworkHealthMonitor) checkHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.health.Endpoints = make([]EndpointHealth, len(m.endpoints))
	for i, ep := range m.endpoints {
		// Simulate health check (in real implementation would ping endpoint)
		m.health.Endpoints[i] = EndpointHealth{
			URL:    ep.URL,
			Status: "healthy", // Simplified for testing
		}
	}
}

// GetHealth returns current network health
func (m *NetworkHealthMonitor) GetHealth() *NetworkHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.health
}

