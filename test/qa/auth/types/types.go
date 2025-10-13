// Package types provides shared type definitions for the authentication test framework.
// These types are used across testenv, mocks, and test packages to avoid circular dependencies.
package types

// RPCEndpoint represents an RPC endpoint configuration
type RPCEndpoint struct {
	URL      string `json:"url"`
	Priority int    `json:"priority"`
	Timeout  int    `json:"timeout"`
}

// TestnetConfig represents testnet configuration
type TestnetConfig struct {
	Sepolia SepoliaConfig `json:"sepolia"`
}

// SepoliaConfig represents Sepolia testnet configuration
type SepoliaConfig struct {
	ChainID        string        `json:"chainId"`
	Name           string        `json:"name"`
	RPCEndpoints   []RPCEndpoint `json:"rpcEndpoints"`
	FallbackToMock bool          `json:"fallbackToMock"`
	MaxRetries     int           `json:"maxRetries"`
	RetryBackoff   string        `json:"retryBackoff"`
}

