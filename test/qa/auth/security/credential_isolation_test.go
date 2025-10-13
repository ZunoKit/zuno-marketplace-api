package security

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/fixtures"
	"github.com/quangdang46/NFT-Marketplace/test/qa/auth/testenv"
)

// TC-SEC-001-01: Mainnet Connection Prevention
// Test: Verify system rejects mainnet RPC endpoints in test mode
// Priority: P1 (Critical Security)
// TDD Phase: RED → Write this test first
func TestMainnetConnectionPrevention(t *testing.T) {
	// GIVEN: Test environment with TESTNET_ONLY_MODE=true
	env := testenv.SetupQAEnvironment(t)
	defer env.Cleanup()

	// WHEN: Attempting to connect to mainnet RPC
	mainnetEndpoints := []string{
		"https://mainnet.infura.io/v3/key",
		"https://eth-mainnet.g.alchemy.com/v2/key",
		"https://cloudflare-eth.com",
	}

	// THEN: All mainnet connections should be rejected
	for _, endpoint := range mainnetEndpoints {
		err := env.BlockchainClient.Connect(endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mainnet connections not allowed in test mode")
	}
}

// TC-SEC-001-02: Production Key Detection
// Test: CI pipeline detects production keys in test code
// Priority: P1 (Critical Security)
// TDD Phase: RED → Write this test first
func TestProductionKeyDetection(t *testing.T) {
	// GIVEN: Test configuration validator
	validator := NewCredentialValidator()

	// Find project root directory
	projectRoot, err := findProjectRoot()
	assert.NoError(t, err, "Failed to find project root")

	// WHEN: Scanning test configuration files
	testFiles := []string{
		filepath.Join(projectRoot, "test/qa/config/.env.qa.example"),
		filepath.Join(projectRoot, "test/qa/auth/testenv/setup.go"),
	}

	// THEN: No production patterns should be found
	for _, file := range testFiles {
		violations, err := validator.ScanFile(file)
		assert.NoError(t, err, "Failed to scan file: %s", file)
		assert.Empty(t, violations, "Production credentials found in test file: %s", file)
	}

	// AND: Production key patterns are correctly detected
	testConfig := map[string]string{
		"JWT_SECRET":  "production_secret_do_not_use",
		"MAINNET_RPC": "https://mainnet.infura.io/v3/real_key",
	}
	violations := validator.ValidateConfig(testConfig)
	assert.NotEmpty(t, violations, "Validator failed to detect production keys")
}

// findProjectRoot finds the project root by looking for go.mod
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree until we find go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding go.mod
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// TC-SEC-001-03: Environment Variable Validation
// Test: Test environment validates all required security flags
// Priority: P1 (Critical Security)
// TDD Phase: RED → Write this test first
func TestEnvironmentVariableValidation(t *testing.T) {
	testCases := []struct {
		name        string
		envVars     map[string]string
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid_qa_config",
			envVars: map[string]string{
				"ENV":                 "qa",
				"TEST_MODE":           "true",
				"TESTNET_ONLY_MODE":   "true",
				"PRODUCTION_DB_GUARD": "true",
			},
			shouldError: false,
		},
		{
			name: "missing_test_mode",
			envVars: map[string]string{
				"ENV": "qa",
			},
			shouldError: true,
			errorMsg:    "TEST_MODE must be true",
		},
		{
			name: "production_db_guard_disabled",
			envVars: map[string]string{
				"ENV":                 "qa",
				"TEST_MODE":           "true",
				"TESTNET_ONLY_MODE":   "true",
				"PRODUCTION_DB_GUARD": "false",
			},
			shouldError: true,
			errorMsg:    "PRODUCTION_DB_GUARD must be enabled",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			validator := NewEnvironmentValidator()
			err := validator.Validate(tc.envVars)

			if tc.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TC-SEC-001-04: Wallet Private Key Isolation
// Test: Test wallets use Sepolia testnet and are cryptographically secure
// Priority: P1 (Critical Security)
// TDD Phase: RED → Write this test first
func TestWalletPrivateKeyIsolation(t *testing.T) {
	// GIVEN: Test wallet generator
	testWallet := fixtures.GenerateTestWallet(t)

	// THEN: Wallet should use Sepolia chain ID (testnet only)
	assert.Equal(t, uint64(11155111), testWallet.ChainID, "Test wallet must use Sepolia testnet")

	// AND: Address should be valid Ethereum address
	assert.NotEmpty(t, testWallet.Address)
	assert.True(t, len(testWallet.Address) == 42, "Address should be 42 characters (0x + 40 hex)")
	assert.Contains(t, testWallet.Address, "0x", "Address should start with 0x")

	// AND: Should not use production chain IDs
	validator := NewWalletValidator()
	isProductionChain := validator.IsProductionChainID(testWallet.ChainID)
	assert.False(t, isProductionChain, "Test wallet using production chain ID")
}

