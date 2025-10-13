// Package testenv provides test environment setup utilities
// for the authentication flow test framework. It handles
// initialization of PostgreSQL, Redis, and blockchain client
// mocks with proper isolation for QA testing.
package testenv

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/quangdang46/NFT-Marketplace/shared/testutil"
	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/mocks"
	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/types"
)

//go:embed testnet.json
var testnetConfigData []byte

// QAEnvironment represents the complete test environment
type QAEnvironment struct {
	t                 *testing.T
	PostgresContainer *testutil.TestDatabase
	RedisContainer    interface{}
	RedisURL          string
	BlockchainClient  *mocks.BlockchainClient
	AuthService       interface{}
	Config            *types.TestnetConfig
}

// SetupQAEnvironment initializes the complete QA test environment
func SetupQAEnvironment(t *testing.T) *QAEnvironment {
	t.Helper()

	ctx := context.Background()

	// Load testnet configuration
	config, err := LoadTestnetConfig()
	if err != nil {
		t.Fatalf("Failed to load testnet config: %v", err)
	}

	// Setup PostgreSQL testcontainer
	testDB, err := testutil.SetupTestPostgres(ctx)
	if err != nil {
		t.Fatalf("Failed to setup test postgres: %v", err)
	}

	// Setup Redis testcontainer
	redisContainer, redisURL, err := testutil.SetupTestRedis(ctx)
	if err != nil {
		testDB.Cleanup(ctx)
		t.Fatalf("Failed to setup test redis: %v", err)
	}

	// Initialize blockchain client with testnet configuration
	blockchainClient := mocks.NewBlockchainClient(config.Sepolia.RPCEndpoints)

	env := &QAEnvironment{
		t:                 t,
		PostgresContainer: testDB,
		RedisContainer:    redisContainer,
		RedisURL:          redisURL,
		BlockchainClient:  blockchainClient,
		Config:            config,
	}

	return env
}

// Cleanup tears down the test environment
func (env *QAEnvironment) Cleanup() {
	ctx := context.Background()

	if env.PostgresContainer != nil {
		env.PostgresContainer.Cleanup(ctx)
	}

	if env.RedisContainer != nil {
		// Type assert and terminate Redis container
		if container, ok := env.RedisContainer.(interface{ Terminate(context.Context) error }); ok {
			container.Terminate(ctx)
		}
	}
}

// LoadTestnetConfig loads testnet configuration from embedded file
func LoadTestnetConfig() (*types.TestnetConfig, error) {
	var config types.TestnetConfig
	if err := json.Unmarshal(testnetConfigData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse testnet config: %w", err)
	}

	// Validate configuration
	if err := validateTestnetConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid testnet config: %w", err)
	}

	return &config, nil
}

// validateTestnetConfig validates testnet configuration
func validateTestnetConfig(config *types.TestnetConfig) error {
	// Validate Sepolia configuration
	if config.Sepolia.ChainID != "eip155:11155111" {
		return fmt.Errorf("invalid Sepolia chain ID: %s (expected eip155:11155111)", config.Sepolia.ChainID)
	}

	if len(config.Sepolia.RPCEndpoints) == 0 {
		return fmt.Errorf("no RPC endpoints configured for Sepolia")
	}

	// Validate each endpoint
	for i, endpoint := range config.Sepolia.RPCEndpoints {
		if endpoint.URL == "" {
			return fmt.Errorf("RPC endpoint %d has empty URL", i)
		}
		if endpoint.Priority < 1 {
			return fmt.Errorf("RPC endpoint %d has invalid priority: %d", i, endpoint.Priority)
		}
		// Ensure it's a testnet endpoint
		endpointLower := strings.ToLower(endpoint.URL)
		if strings.Contains(endpointLower, "mainnet") {
			return fmt.Errorf("mainnet endpoint not allowed: %s", endpoint.URL)
		}
		// Ensure it has Sepolia in the URL or is a known testnet endpoint
		if !strings.Contains(endpointLower, "sepolia") && !strings.Contains(endpointLower, "11155111") {
			// Allow known testnet providers
			knownTestnetProviders := []string{"rpc.sepolia.org", "ethereum-sepolia-rpc"}
			isKnownTestnet := false
			for _, provider := range knownTestnetProviders {
				if strings.Contains(endpointLower, provider) {
					isKnownTestnet = true
					break
				}
			}
			if !isKnownTestnet {
				return fmt.Errorf("endpoint does not appear to be Sepolia testnet: %s", endpoint.URL)
			}
		}
	}

	return nil
}

