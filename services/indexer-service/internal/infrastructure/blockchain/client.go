package blockchain

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
)

// Client implements the BlockchainClient interface for Ethereum-compatible chains
type Client struct {
	chainID            string
	ethClient          *ethclient.Client
	rpcClient          *rpc.Client
	confirmationBlocks int
	rpcURL             string
}

// NewClient creates a new blockchain client
func NewClient(chainID, rpcURL string, confirmationBlocks map[string]int) (*Client, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC URL cannot be empty for chain %s", chainID)
	}

	rpcClient, err := rpc.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to dial RPC for chain %s: %w", chainID, err)
	}

	ethClient := ethclient.NewClient(rpcClient)

	// Get confirmation blocks for this chain
	chainConfirmations := confirmationBlocks[chainID]
	if chainConfirmations == 0 {
		chainConfirmations = 12 // Default for mainnet
	}

	client := &Client{
		chainID:            chainID,
		ethClient:          ethClient,
		rpcClient:          rpcClient,
		confirmationBlocks: chainConfirmations,
		rpcURL:             rpcURL,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if _, err := client.GetLatestBlock(ctx); err != nil {
		return nil, fmt.Errorf("failed to test connection for chain %s: %w", chainID, err)
	}

	return client, nil
}

// GetLatestBlock returns the latest block number
func (c *Client) GetLatestBlock(ctx context.Context) (*big.Int, error) {
	header, err := c.ethClient.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest block header: %w", err)
	}
	return header.Number, nil
}

// GetBlockByNumber returns block information
func (c *Client) GetBlockByNumber(ctx context.Context, blockNumber *big.Int) (*domain.BlockInfo, error) {
	block, err := c.ethClient.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", blockNumber.String(), err)
	}

	return &domain.BlockInfo{
		Number:    block.Number(),
		Hash:      block.Hash().Hex(),
		Timestamp: time.Unix(int64(block.Time()), 0),
	}, nil
}

// GetLogs returns logs for the specified filter
func (c *Client) GetLogs(ctx context.Context, filter *domain.LogFilter) ([]*domain.Log, error) {
	ethFilter := ethereum.FilterQuery{
		FromBlock: filter.FromBlock,
		ToBlock:   filter.ToBlock,
	}

	// Convert addresses
	if len(filter.Addresses) > 0 {
		addresses := make([]common.Address, len(filter.Addresses))
		for i, addr := range filter.Addresses {
			addresses[i] = common.HexToAddress(addr)
		}
		ethFilter.Addresses = addresses
	}

	// Convert topics
	if len(filter.Topics) > 0 {
		topics := make([][]common.Hash, len(filter.Topics))
		for i, topic := range filter.Topics {
			if topic != "" {
				topics[i] = []common.Hash{common.HexToHash(topic)}
			}
		}
		ethFilter.Topics = topics
	}

	logs, err := c.ethClient.FilterLogs(ctx, ethFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	domainLogs := make([]*domain.Log, len(logs))
	for i, log := range logs {
		topics := make([]string, len(log.Topics))
		for j, topic := range log.Topics {
			topics[j] = topic.Hex()
		}

		domainLogs[i] = &domain.Log{
			Address:     log.Address.Hex(),
			Topics:      topics,
			Data:        fmt.Sprintf("0x%x", log.Data),
			BlockNumber: new(big.Int).SetUint64(log.BlockNumber),
			TxHash:      log.TxHash.Hex(),
			LogIndex:    int(log.Index),
			BlockHash:   log.BlockHash.Hex(),
			Removed:     log.Removed,
		}
	}

	return domainLogs, nil
}

// GetConfirmations returns the number of confirmations for a block
func (c *Client) GetConfirmations(ctx context.Context, blockNumber *big.Int) (int, error) {
	latestBlock, err := c.GetLatestBlock(ctx)
	if err != nil {
		return 0, err
	}

	if blockNumber.Cmp(latestBlock) > 0 {
		return 0, nil // Block is in the future
	}

	confirmations := new(big.Int).Sub(latestBlock, blockNumber)
	return int(confirmations.Int64()) + 1, nil // +1 because the block itself is the first confirmation
}

// Close closes the client connections
func (c *Client) Close() {
	if c.ethClient != nil {
		c.ethClient.Close()
	}
	if c.rpcClient != nil {
		c.rpcClient.Close()
	}
}

// GetChainID returns the client's chain ID
func (c *Client) GetChainID() string {
	return c.chainID
}

// IsHealthy checks if the client connection is healthy
func (c *Client) IsHealthy(ctx context.Context) error {
	_, err := c.GetLatestBlock(ctx)
	return err
}

// GetCollectionCreatedEventFilter creates a filter for CollectionCreated events
func (c *Client) GetCollectionCreatedEventFilter(factoryAddress string, fromBlock *big.Int) *domain.LogFilter {
	// CollectionCreated event signature
	// event CollectionCreated(address indexed collection, address indexed creator, string name, string symbol, uint8 collectionType)
	collectionCreatedTopic := "0x" + "4d72fe0577a3a3f7da968d7b892779dde102519c25c1838b6653ccc4b0b96d2e" // This is a placeholder, needs actual event signature
	
	return &domain.LogFilter{
		FromBlock: fromBlock,
		ToBlock:   nil, // Latest block
		Addresses: []string{factoryAddress},
		Topics:    []string{collectionCreatedTopic},
	}
}

// ParseCollectionCreatedLog parses a CollectionCreated event log
func (c *Client) ParseCollectionCreatedLog(log *domain.Log) (*domain.CollectionCreatedEvent, error) {
	if len(log.Topics) < 3 {
		return nil, fmt.Errorf("invalid CollectionCreated log: insufficient topics")
	}

	// Extract indexed parameters from topics
	collectionAddress := c.addressFromTopic(log.Topics[1])
	creator := c.addressFromTopic(log.Topics[2])

	// Parse non-indexed parameters from data
	// This is a simplified version - in practice, you'd use ABI decoding
	// For now, we'll return basic information
	event := &domain.CollectionCreatedEvent{
		CollectionAddress: collectionAddress,
		Creator:          creator,
		Name:             "", // Would be parsed from log.Data using ABI
		Symbol:           "", // Would be parsed from log.Data using ABI
		CollectionType:   "ERC721", // Default assumption, would be parsed from data
		MaxSupply:        big.NewInt(0), // Would be parsed from data
		RoyaltyRecipient: creator, // Default assumption
		RoyaltyPercentage: 0, // Would be parsed from data
	}

	return event, nil
}

// addressFromTopic extracts an address from a log topic
func (c *Client) addressFromTopic(topic string) string {
	if len(topic) != 66 { // 0x + 64 hex chars
		return ""
	}
	// Address is the last 20 bytes (40 hex chars) of the topic
	return "0x" + topic[26:]
}

// GetBlockRange returns a range of blocks for batch processing
func (c *Client) GetBlockRange(ctx context.Context, fromBlock, toBlock *big.Int, batchSize int64) ([]*big.Int, error) {
	if fromBlock.Cmp(toBlock) > 0 {
		return nil, fmt.Errorf("fromBlock (%s) cannot be greater than toBlock (%s)", fromBlock.String(), toBlock.String())
	}

	var blocks []*big.Int
	current := new(big.Int).Set(fromBlock)
	
	for current.Cmp(toBlock) <= 0 {
		blocks = append(blocks, new(big.Int).Set(current))
		current.Add(current, big.NewInt(1))
		
		// Limit batch size to prevent memory issues
		if int64(len(blocks)) >= batchSize {
			break
		}
	}

	return blocks, nil
}