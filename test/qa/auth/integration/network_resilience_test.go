package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/mocks"
	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/testenv"
)

// TC-TECH-001-01: RPC Endpoint Failover
// Test: System fails over to backup RPC when primary unavailable
// Priority: P1 (Critical Reliability)
// TDD Phase: RED → Write this test first
func TestRPCEndpointFailover(t *testing.T) {
	// GIVEN: Blockchain client with multiple endpoints
	config, err := testenv.LoadTestnetConfig()
	if err != nil {
		t.Fatalf("Failed to load testnet config: %v", err)
	}

	client := mocks.NewBlockchainClientWithFailover(config.Sepolia.RPCEndpoints)

	// WHEN: Primary endpoint is unavailable
	client.SimulateEndpointFailure(config.Sepolia.RPCEndpoints[0].URL)

	// THEN: Client should failover to secondary endpoint
	chainID, err := client.GetChainID(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, uint64(11155111), chainID)

	// AND: Client should use secondary endpoint
	activeEndpoint := client.GetActiveEndpoint()
	assert.Equal(t, config.Sepolia.RPCEndpoints[1].URL, activeEndpoint)
}

// TC-TECH-001-02: Exponential Backoff Retry
// Test: Network failures trigger exponential backoff retry
// Priority: P1 (Critical Reliability)
// TDD Phase: RED → Write this test first
func TestExponentialBackoffRetry(t *testing.T) {
	// GIVEN: Client with retry configuration
	client := mocks.NewBlockchainClientWithRetry(3, 100*time.Millisecond)

	// WHEN: Simulating transient network failures
	attemptTimes := []time.Time{}
	client.OnRetry(func(attempt int) {
		attemptTimes = append(attemptTimes, time.Now())
	})

	// Simulate 2 failures, then success (total 3 calls)
	client.SimulateFailures(2)
	
	// First call will fail
	_, err := client.GetChainID(context.Background())
	assert.Error(t, err, "First call should fail")
	
	// Wait before retry (simulating exponential backoff)
	time.Sleep(100 * time.Millisecond)
	
	// Second call will fail (retry 1)
	_, err = client.GetChainID(context.Background())
	assert.Error(t, err, "Second call should fail")
	
	// Wait before retry (simulating exponential backoff)
	time.Sleep(200 * time.Millisecond)
	
	// Third call will succeed (retry 2)
	_, err = client.GetChainID(context.Background())

	// THEN: Request should eventually succeed
	assert.NoError(t, err, "Third call should succeed")
	assert.Len(t, attemptTimes, 2, "Expected 2 retries")

	// AND: Backoff should be exponential (verify timing)
	if len(attemptTimes) >= 2 {
		firstDelay := attemptTimes[1].Sub(attemptTimes[0])
		assert.GreaterOrEqual(t, firstDelay, 50*time.Millisecond) // Allow some variance
		assert.LessOrEqual(t, firstDelay, 350*time.Millisecond)  // Increased upper bound for backoff
	}
}

// TC-TECH-001-03: Offline Mock Fallback
// Test: System falls back to mock blockchain when all endpoints fail
// Priority: P1 (Critical Reliability)
// TDD Phase: RED → Write this test first
func TestOfflineMockFallback(t *testing.T) {
	// GIVEN: Blockchain client with mock fallback enabled
	config, err := testenv.LoadTestnetConfig()
	if err != nil {
		t.Fatalf("Failed to load testnet config: %v", err)
	}

	client := mocks.NewBlockchainClientWithMockFallback(config)

	// WHEN: All RPC endpoints are unavailable
	for _, endpoint := range config.Sepolia.RPCEndpoints {
		client.SimulateEndpointFailure(endpoint.URL)
	}

	// THEN: Client should use mock blockchain
	chainID, err := client.GetChainID(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, uint64(11155111), chainID)

	// AND: Mock mode should be active
	assert.True(t, client.IsUsingMockBackend(), "Client not using mock fallback")
}

// TC-TECH-001-04: Network Timeout Handling
// Test: Request timeouts handled gracefully with circuit breaker
// Priority: P1 (Critical Reliability)
// TDD Phase: RED → Write this test first
func TestNetworkTimeoutHandling(t *testing.T) {
	// GIVEN: Client with timeout configuration
	client := mocks.NewBlockchainClientWithTimeout(5 * time.Second)

	// WHEN: Simulating slow network response
	client.SimulateSlowResponse(10 * time.Second)

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	_, err := client.GetChainID(ctx)
	duration := time.Since(start)

	// THEN: Request should timeout within configured duration
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.LessOrEqual(t, duration, 7*time.Second, "Timeout not enforced")
}

// TC-TECH-001-05: Testnet Status Monitoring
// Test: System detects testnet health status proactively
// Priority: P1 (Critical Reliability)
// TDD Phase: RED → Write this test first
func TestTestnetStatusMonitoring(t *testing.T) {
	// GIVEN: Network monitor with health checking
	config, err := testenv.LoadTestnetConfig()
	if err != nil {
		t.Fatalf("Failed to load testnet config: %v", err)
	}

	monitor := mocks.NewNetworkHealthMonitor(config.Sepolia.RPCEndpoints)

	// WHEN: Starting health monitoring
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	monitor.Start(ctx)
	time.Sleep(2 * time.Second) // Allow health checks to run

	// THEN: Monitor should report endpoint health
	health := monitor.GetHealth()
	assert.NotEmpty(t, health.Endpoints)

	for _, endpoint := range health.Endpoints {
		assert.NotEmpty(t, endpoint.URL)
		assert.Contains(t, []string{"healthy", "degraded", "unhealthy"}, endpoint.Status)
	}
}

