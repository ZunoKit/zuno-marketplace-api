package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
)

// TxValidator validates blockchain transactions
type TxValidator struct {
	clients map[string]*ethclient.Client // chainID -> client
	cache   *TxCache                     // Cache validated transactions
}

// TxCache caches validated transactions
type TxCache struct {
	entries map[string]*CacheEntry
	ttl     time.Duration
}

// CacheEntry represents a cached transaction validation
type CacheEntry struct {
	TxHash      string
	Valid       bool
	Contract    string
	BlockNumber *big.Int
	Status      uint64
	ValidatedAt time.Time
	Error       string
}

// NewTxValidator creates a new transaction validator
func NewTxValidator(rpcURLs map[string]string) (*TxValidator, error) {
	clients := make(map[string]*ethclient.Client)

	for chainID, rpcURL := range rpcURLs {
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %w", chainID, err)
		}
		clients[chainID] = client
	}

	return &TxValidator{
		clients: clients,
		cache: &TxCache{
			entries: make(map[string]*CacheEntry),
			ttl:     5 * time.Minute,
		},
	}, nil
}

// ValidateTransaction validates a transaction exists on-chain and matches expected criteria
func (v *TxValidator) ValidateTransaction(ctx context.Context, req *ValidateTxRequest) (*ValidateTxResponse, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s", req.ChainID, req.TxHash)
	if cached, ok := v.cache.Get(cacheKey); ok {
		return &ValidateTxResponse{
			Valid:       cached.Valid,
			Contract:    cached.Contract,
			BlockNumber: cached.BlockNumber,
			Status:      cached.Status,
			Error:       cached.Error,
		}, nil
	}

	// Get client for chain
	client, ok := v.clients[req.ChainID]
	if !ok {
		return nil, fmt.Errorf("no client configured for chain %s", req.ChainID)
	}

	// Get transaction receipt
	txHash := common.HexToHash(req.TxHash)
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		// Transaction not found or not mined yet
		if strings.Contains(err.Error(), "not found") {
			return &ValidateTxResponse{
				Valid: false,
				Error: "transaction not found on chain",
			}, nil
		}
		return nil, fmt.Errorf("failed to get transaction receipt: %w", err)
	}

	// Get transaction details
	tx, _, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get transaction: %w", err)
	}

	// Validate transaction
	response := &ValidateTxResponse{
		Valid:       receipt.Status == 1, // 1 = success, 0 = failed
		Contract:    receipt.ContractAddress.Hex(),
		BlockNumber: receipt.BlockNumber,
		Status:      receipt.Status,
	}

	// Additional validations based on intent type
	if req.IntentType == domain.IntentKindCollection {
		// For collection creation, contract address should be present
		if receipt.ContractAddress == (common.Address{}) {
			response.Valid = false
			response.Error = "no contract deployed in transaction"
		}
	} else if req.IntentType == domain.IntentKindMint {
		// For minting, transaction should be to a contract
		if tx.To() == nil {
			response.Valid = false
			response.Error = "mint transaction has no recipient"
		} else if req.ExpectedContract != "" {
			// Validate it's to the expected contract
			if !strings.EqualFold(tx.To().Hex(), req.ExpectedContract) {
				response.Valid = false
				response.Error = fmt.Sprintf("transaction to wrong contract: expected %s, got %s",
					req.ExpectedContract, tx.To().Hex())
			}
		}
	}

	// Check transaction wasn't too old (prevent replay of old txs)
	if req.MaxAge > 0 {
		blockTime, err := v.getBlockTime(ctx, client, receipt.BlockNumber)
		if err == nil {
			age := time.Since(time.Unix(int64(blockTime), 0))
			if age > req.MaxAge {
				response.Valid = false
				response.Error = fmt.Sprintf("transaction too old: %v", age)
			}
		}
	}

	// Check gas price isn't suspiciously low (potential spam)
	if req.MinGasPrice != nil && tx.GasPrice().Cmp(req.MinGasPrice) < 0 {
		response.Valid = false
		response.Error = "transaction gas price too low"
	}

	// Cache the result
	v.cache.Set(cacheKey, &CacheEntry{
		TxHash:      req.TxHash,
		Valid:       response.Valid,
		Contract:    response.Contract,
		BlockNumber: response.BlockNumber,
		Status:      response.Status,
		ValidatedAt: time.Now(),
		Error:       response.Error,
	})

	return response, nil
}

// ValidatePendingTransaction checks if a transaction is pending
func (v *TxValidator) ValidatePendingTransaction(ctx context.Context, chainID, txHash string) (bool, error) {
	client, ok := v.clients[chainID]
	if !ok {
		return false, fmt.Errorf("no client configured for chain %s", chainID)
	}

	hash := common.HexToHash(txHash)

	// Check if transaction is pending
	tx, isPending, err := client.TransactionByHash(ctx, hash)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, fmt.Errorf("failed to get transaction: %w", err)
	}

	// If pending, validate basic properties
	if isPending {
		// Check transaction has valid nonce, gas, etc.
		if tx.Gas() == 0 {
			return false, fmt.Errorf("invalid gas limit")
		}
		if tx.GasPrice() == nil || tx.GasPrice().Sign() <= 0 {
			return false, fmt.Errorf("invalid gas price")
		}
		return true, nil
	}

	// Transaction is mined, not pending
	return false, nil
}

// getBlockTime gets the timestamp of a block
func (v *TxValidator) getBlockTime(ctx context.Context, client *ethclient.Client, blockNumber *big.Int) (uint64, error) {
	block, err := client.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return 0, err
	}
	return block.Time(), nil
}

// Get retrieves a cached entry
func (c *TxCache) Get(key string) (*CacheEntry, bool) {
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	// Check if entry is expired
	if time.Since(entry.ValidatedAt) > c.ttl {
		delete(c.entries, key)
		return nil, false
	}

	return entry, true
}

// Set stores an entry in cache
func (c *TxCache) Set(key string, entry *CacheEntry) {
	c.entries[key] = entry

	// Simple cleanup: remove old entries periodically
	if len(c.entries) > 1000 {
		c.cleanup()
	}
}

// cleanup removes expired entries
func (c *TxCache) cleanup() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.Sub(entry.ValidatedAt) > c.ttl {
			delete(c.entries, key)
		}
	}
}

// ValidateTxRequest represents a transaction validation request
type ValidateTxRequest struct {
	ChainID          string
	TxHash           string
	IntentType       domain.IntentKind
	ExpectedContract string        // For mint transactions
	MaxAge           time.Duration // Maximum age of transaction
	MinGasPrice      *big.Int      // Minimum acceptable gas price
}

// ValidateTxResponse represents a transaction validation response
type ValidateTxResponse struct {
	Valid       bool
	Contract    string // Contract address (for deployments)
	BlockNumber *big.Int
	Status      uint64
	Error       string
}
