package security

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// CredentialValidator validates that test configurations don't contain production credentials
type CredentialValidator struct {
	productionPatterns []string
	mainnetPatterns    []string
	mainnetRegexps     []*regexp.Regexp
}

// NewCredentialValidator creates a new credential validator
func NewCredentialValidator() *CredentialValidator {
	return &CredentialValidator{
		productionPatterns: []string{
			"production",
			"prod",
			"mainnet",
		},
		mainnetPatterns: []string{
			"mainnet.infura.io",
			"eth-mainnet",
			"cloudflare-eth.com",
		},
		// Regex patterns for exact chain ID matching
		mainnetRegexps: []*regexp.Regexp{
			regexp.MustCompile(`\beip155:1\b`),    // Ethereum mainnet only (not 11, 111, etc.)
			regexp.MustCompile(`\beip155:56\b`),   // BSC mainnet
			regexp.MustCompile(`\beip155:137\b`),  // Polygon mainnet
		},
	}
}

// ScanFile scans a file for production credential patterns
func (v *CredentialValidator) ScanFile(filepath string) ([]string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filepath, err)
	}
	defer file.Close()

	var violations []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lineLower := strings.ToLower(line)

		// Skip comments
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}

		// Check for mainnet string patterns (case-insensitive)
		for _, pattern := range v.mainnetPatterns {
			if strings.Contains(lineLower, pattern) {
				violations = append(violations, fmt.Sprintf("%s:%d: mainnet reference found: %s", filepath, lineNum, pattern))
			}
		}

		// Check for mainnet chain IDs using regex (exact match)
		for _, pattern := range v.mainnetRegexps {
			if pattern.MatchString(line) {
				violations = append(violations, fmt.Sprintf("%s:%d: mainnet chain ID found: %s", filepath, lineNum, pattern.String()))
			}
		}

		// Check for production secret patterns
		if strings.Contains(lineLower, "secret") && strings.Contains(lineLower, "production") {
			violations = append(violations, fmt.Sprintf("%s:%d: production secret pattern found", filepath, lineNum))
		}
	}

	if err := scanner.Err(); err != nil {
		return violations, fmt.Errorf("error scanning file: %w", err)
	}

	return violations, nil
}

// ValidateConfig validates a configuration map for production patterns
func (v *CredentialValidator) ValidateConfig(config map[string]string) []string {
	var violations []string

	for key, value := range config {
		valueLower := strings.ToLower(value)

		// Check for production patterns in values
		for _, pattern := range v.productionPatterns {
			if strings.Contains(valueLower, pattern) {
				violations = append(violations, fmt.Sprintf("config key '%s' contains production pattern: %s", key, pattern))
			}
		}

		// Check for mainnet string patterns
		for _, pattern := range v.mainnetPatterns {
			if strings.Contains(valueLower, pattern) {
				violations = append(violations, fmt.Sprintf("config key '%s' contains mainnet pattern: %s", key, pattern))
			}
		}

		// Check for mainnet chain IDs using regex (exact match)
		for _, pattern := range v.mainnetRegexps {
			if pattern.MatchString(value) {
				violations = append(violations, fmt.Sprintf("config key '%s' contains mainnet chain ID: %s", key, pattern.String()))
			}
		}
	}

	return violations
}

// EnvironmentValidator validates environment configuration for test mode
type EnvironmentValidator struct {
	requiredFlags map[string]string
}

// NewEnvironmentValidator creates a new environment validator
func NewEnvironmentValidator() *EnvironmentValidator {
	return &EnvironmentValidator{
		requiredFlags: map[string]string{
			"TEST_MODE":           "true",
			"TESTNET_ONLY_MODE":   "true",
			"PRODUCTION_DB_GUARD": "true",
		},
	}
}

// Validate validates environment variables for test mode
func (v *EnvironmentValidator) Validate(envVars map[string]string) error {
	// Check required flags in specific order to ensure consistent error messages
	orderedKeys := []string{"TEST_MODE", "TESTNET_ONLY_MODE", "PRODUCTION_DB_GUARD"}
	
	for _, key := range orderedKeys {
		expectedValue := v.requiredFlags[key]
		actualValue, exists := envVars[key]
		
		if !exists {
			// Custom message for PRODUCTION_DB_GUARD
			if key == "PRODUCTION_DB_GUARD" {
				return fmt.Errorf("PRODUCTION_DB_GUARD must be enabled")
			}
			return fmt.Errorf("%s must be true", key)
		}
		if strings.ToLower(actualValue) != strings.ToLower(expectedValue) {
			// Custom message for PRODUCTION_DB_GUARD
			if key == "PRODUCTION_DB_GUARD" {
				return fmt.Errorf("PRODUCTION_DB_GUARD must be enabled")
			}
			return fmt.Errorf("%s must be true", key)
		}
	}

	// Ensure we're not in production environment
	if env, exists := envVars["ENV"]; exists {
		if strings.ToLower(env) == "production" || strings.ToLower(env) == "prod" {
			return fmt.Errorf("ENV cannot be production in test mode")
		}
	}

	// Ensure mainnet connections are disabled
	if allow, exists := envVars["ALLOW_MAINNET_CONNECTIONS"]; exists {
		if strings.ToLower(allow) == "true" {
			return fmt.Errorf("ALLOW_MAINNET_CONNECTIONS must be false in test mode")
		}
	}

	return nil
}

// WalletValidator validates wallet configurations
type WalletValidator struct {
	productionKeyPatterns []*regexp.Regexp
}

// NewWalletValidator creates a new wallet validator
func NewWalletValidator() *WalletValidator {
	return &WalletValidator{
		productionKeyPatterns: []*regexp.Regexp{
			regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`), // Real production keys (full 64 hex chars without test marker)
		},
	}
}

// IsProductionKey checks if a private key matches production patterns
func (v *WalletValidator) IsProductionKey(privateKey string) bool {
	// Test keys should have "test" marker or be clearly synthetic
	if strings.Contains(strings.ToLower(privateKey), "test") {
		return false
	}

	// Check against production key patterns
	for _, pattern := range v.productionKeyPatterns {
		if pattern.MatchString(privateKey) {
			// If it matches production pattern and doesn't have test marker, it's production
			return true
		}
	}

	return false
}

// IsProductionChainID checks if a chain ID is a production chain
func (v *WalletValidator) IsProductionChainID(chainID uint64) bool {
	productionChains := []uint64{
		1,   // Ethereum mainnet
		56,  // BSC mainnet
		137, // Polygon mainnet
	}

	for _, prodChain := range productionChains {
		if chainID == prodChain {
			return true
		}
	}
	return false
}

// DatabaseConnectionValidator validates database connection strings
type DatabaseConnectionValidator struct {
	productionHosts []string
	productionDBs   []string
}

// NewDatabaseConnectionValidator creates a new database connection validator
func NewDatabaseConnectionValidator() *DatabaseConnectionValidator {
	return &DatabaseConnectionValidator{
		productionHosts: []string{
			"prod-db",
			"production",
			"rds.amazonaws.com",
			".prod.",
		},
		productionDBs: []string{
			"marketplace",
			"nft_prod",
			"production",
		},
	}
}

// ValidateConnectionString validates a database connection string
func (v *DatabaseConnectionValidator) ValidateConnectionString(connString string) bool {
	connLower := strings.ToLower(connString)

	// Check for production host patterns
	for _, host := range v.productionHosts {
		if strings.Contains(connLower, host) {
			return false
		}
	}

	// Check for production database names
	for _, db := range v.productionDBs {
		if strings.Contains(connLower, "dbname="+db) {
			return false
		}
	}

	// Check for production schema in search_path
	if strings.Contains(connLower, "search_path=auth") && !strings.Contains(connLower, "auth_test") {
		return false
	}

	// Allow localhost and test databases
	return strings.Contains(connLower, "localhost") || strings.Contains(connLower, "testdb")
}

