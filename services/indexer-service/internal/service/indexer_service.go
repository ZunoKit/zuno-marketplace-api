package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/infrastructure/blockchain"
)

const (
	// CollectionCreated event signature (keccak256 hash)
	CollectionCreatedSignature = "0x4d72fe0577a3a3f7da968d7b892779dde102519c25c1838b6653ccc4b0b96d2e" // Placeholder
	
	// Batch processing settings
	MaxBlockBatchSize = 100
	MaxRetries       = 3
	RetryDelay       = 5 * time.Second
)

type IndexerService struct {
	eventRepo          domain.EventRepository
	checkpointRepo     domain.CheckpointRepository
	publisher          domain.EventPublisher
	blockchainClients  map[string]*blockchain.Client
	factoryContracts   map[string]string // chainID -> factory contract address
	pollingInterval    time.Duration
	
	// Control channels
	stopChan   chan struct{}
	errorChan  chan error
	wg         sync.WaitGroup
	mu         sync.RWMutex
	isRunning  bool
}

// NewIndexerService creates a new indexer service
func NewIndexerService(
	eventRepo domain.EventRepository,
	checkpointRepo domain.CheckpointRepository,
	publisher domain.EventPublisher,
	blockchainClients map[string]*blockchain.Client,
	factoryContracts map[string]string,
	pollingInterval time.Duration,
) *IndexerService {
	return &IndexerService{
		eventRepo:         eventRepo,
		checkpointRepo:    checkpointRepo,
		publisher:         publisher,
		blockchainClients: blockchainClients,
		factoryContracts:  factoryContracts,
		pollingInterval:   pollingInterval,
		stopChan:          make(chan struct{}),
		errorChan:         make(chan error, len(blockchainClients)),
	}
}

// Start begins the indexing process for all configured chains
func (s *IndexerService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("indexer service is already running")
	}
	s.isRunning = true
	s.mu.Unlock()

	fmt.Printf("Starting indexer service for %d chains\n", len(s.blockchainClients))

	// Start indexing for each configured chain
	for chainID := range s.blockchainClients {
		factoryAddress, exists := s.factoryContracts[chainID]
		if !exists || factoryAddress == "" {
			fmt.Printf("Warning: No factory contract configured for chain %s, skipping\n", chainID)
			continue
		}

		s.wg.Add(1)
		go func(chainID, factoryAddress string) {
			defer s.wg.Done()
			s.indexChainLoop(ctx, chainID, factoryAddress)
		}(chainID, factoryAddress)
	}

	// Monitor for errors
	go func() {
		for {
			select {
			case err := <-s.errorChan:
				fmt.Printf("Indexer error: %v\n", err)
			case <-ctx.Done():
				return
			case <-s.stopChan:
				return
			}
		}
	}()

	// Wait for all indexers to finish
	s.wg.Wait()

	s.mu.Lock()
	s.isRunning = false
	s.mu.Unlock()

	return nil
}

// Stop gracefully shuts down the indexing process
func (s *IndexerService) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	fmt.Println("Stopping indexer service...")

	// Signal all goroutines to stop
	close(s.stopChan)

	// Wait for graceful shutdown with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("Indexer service stopped gracefully")
	case <-ctx.Done():
		fmt.Println("Indexer service shutdown timeout")
		return ctx.Err()
	}

	return nil
}

// IndexChain indexes events for a specific chain
func (s *IndexerService) IndexChain(ctx context.Context, chainID string) error {
	client, exists := s.blockchainClients[chainID]
	if !exists {
		return fmt.Errorf("blockchain client not found for chain %s", chainID)
	}

	factoryAddress, exists := s.factoryContracts[chainID]
	if !exists {
		return fmt.Errorf("factory contract not found for chain %s", chainID)
	}

	return s.processChainEvents(ctx, chainID, factoryAddress, client)
}

// indexChainLoop runs the continuous indexing loop for a specific chain
func (s *IndexerService) indexChainLoop(ctx context.Context, chainID, factoryAddress string) {
	client := s.blockchainClients[chainID]
	ticker := time.NewTicker(s.pollingInterval)
	defer ticker.Stop()

	fmt.Printf("Starting indexing loop for chain %s\n", chainID)

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Context cancelled for chain %s\n", chainID)
			return
		case <-s.stopChan:
			fmt.Printf("Stop signal received for chain %s\n", chainID)
			return
		case <-ticker.C:
			if err := s.processChainEvents(ctx, chainID, factoryAddress, client); err != nil {
				s.errorChan <- fmt.Errorf("chain %s indexing error: %w", chainID, err)
				// Continue processing despite errors
			}
		}
	}
}

// processChainEvents processes new events for a specific chain
func (s *IndexerService) processChainEvents(ctx context.Context, chainID, factoryAddress string, client *blockchain.Client) error {
	// Get latest checkpoint
	checkpoint, err := s.checkpointRepo.GetCheckpoint(ctx, chainID)
	if err != nil {
		return fmt.Errorf("failed to get checkpoint: %w", err)
	}

	// Get latest block from blockchain
	latestBlock, err := client.GetLatestBlock(ctx)
	if err != nil {
		return fmt.Errorf("failed to get latest block: %w", err)
	}

	// Calculate next block to process
	nextBlock := new(big.Int).Add(checkpoint.LastBlock, big.NewInt(1))

	// If we're already at the latest block, nothing to do
	if nextBlock.Cmp(latestBlock) > 0 {
		return nil
	}

	// Process blocks in batches to avoid overwhelming the system
	batchSize := int64(MaxBlockBatchSize)
	fromBlock := nextBlock
	
	for fromBlock.Cmp(latestBlock) <= 0 {
		// Calculate batch end block
		toBlock := new(big.Int).Add(fromBlock, big.NewInt(batchSize-1))
		if toBlock.Cmp(latestBlock) > 0 {
			toBlock = new(big.Int).Set(latestBlock)
		}

		fmt.Printf("Processing blocks %s to %s for chain %s\n", fromBlock.String(), toBlock.String(), chainID)

		// Get logs for this batch
		filter := &domain.LogFilter{
			FromBlock: fromBlock,
			ToBlock:   toBlock,
			Addresses: []string{factoryAddress},
			Topics:    []string{CollectionCreatedSignature},
		}

		logs, err := client.GetLogs(ctx, filter)
		if err != nil {
			return fmt.Errorf("failed to get logs for blocks %s-%s: %w", fromBlock.String(), toBlock.String(), err)
		}

		// Process each log
		for _, log := range logs {
			if err := s.processCollectionCreatedLog(ctx, chainID, log, client); err != nil {
				fmt.Printf("Failed to process log %s:%d: %v\n", log.TxHash, log.LogIndex, err)
				// Continue processing other logs
			}
		}

		// Update checkpoint to the last processed block
		blockInfo, err := client.GetBlockByNumber(ctx, toBlock)
		if err != nil {
			return fmt.Errorf("failed to get block info for %s: %w", toBlock.String(), err)
		}

		newCheckpoint := &domain.Checkpoint{
			ChainID:       chainID,
			LastBlock:     toBlock,
			LastBlockHash: blockInfo.Hash,
			UpdatedAt:     time.Now(),
		}

		if err := s.checkpointRepo.UpdateCheckpoint(ctx, newCheckpoint); err != nil {
			return fmt.Errorf("failed to update checkpoint: %w", err)
		}

		// Move to next batch
		fromBlock = new(big.Int).Add(toBlock, big.NewInt(1))

		// Small delay between batches to avoid overwhelming the node
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		case <-s.stopChan:
			return nil
		}
	}

	return nil
}

// processCollectionCreatedLog processes a single CollectionCreated log
func (s *IndexerService) processCollectionCreatedLog(ctx context.Context, chainID string, log *domain.Log, client *blockchain.Client) error {
	// Check confirmations
	confirmations, err := client.GetConfirmations(ctx, log.BlockNumber)
	if err != nil {
		return fmt.Errorf("failed to get confirmations: %w", err)
	}

	// Create raw event
	rawEvent := &domain.RawEvent{
		ChainID:         chainID,
		TxHash:          log.TxHash,
		LogIndex:        log.LogIndex,
		BlockNumber:     log.BlockNumber,
		BlockHash:       log.BlockHash,
		ContractAddress: log.Address,
		EventName:       "CollectionCreated",
		EventSignature:  CollectionCreatedSignature,
		RawData: map[string]interface{}{
			"topics": log.Topics,
			"data":   log.Data,
		},
		Confirmations: confirmations,
		ObservedAt:    time.Now(),
	}

	// Parse the collection created event
	collectionEvent, err := client.ParseCollectionCreatedLog(log)
	if err != nil {
		return fmt.Errorf("failed to parse collection created log: %w", err)
	}

	// Serialize parsed data to JSON
	parsedJSON, err := json.Marshal(collectionEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal parsed event: %w", err)
	}
	rawEvent.ParsedJSON = string(parsedJSON)

	// Store raw event in MongoDB (with deduplication)
	if err := s.eventRepo.StoreRawEvent(ctx, rawEvent); err != nil {
		return fmt.Errorf("failed to store raw event: %w", err)
	}

	// Only publish if event has sufficient confirmations
	requiredConfirmations := s.getRequiredConfirmations(chainID)
	if confirmations >= requiredConfirmations {
		if err := s.publisher.PublishCollectionCreatedEvent(ctx, chainID, rawEvent, collectionEvent); err != nil {
			return fmt.Errorf("failed to publish collection created event: %w", err)
		}
		fmt.Printf("Published CollectionCreated event for %s on chain %s\n", collectionEvent.CollectionAddress, chainID)
	} else {
		fmt.Printf("Event %s:%d has %d confirmations, need %d\n", log.TxHash, log.LogIndex, confirmations, requiredConfirmations)
	}

	return nil
}

// getRequiredConfirmations returns the required number of confirmations for a chain
func (s *IndexerService) getRequiredConfirmations(chainID string) int {
	// This should be configurable per chain
	// For now, using default values
	switch chainID {
	case "eip155-1": // Ethereum Mainnet
		return 12
	case "eip155-137": // Polygon
		return 20
	case "eip155-11155111": // Ethereum Sepolia
		return 3
	case "eip155-80001": // Polygon Mumbai
		return 5
	default:
		return 6 // Default
	}
}

// HealthCheck performs a health check on the indexer service
func (s *IndexerService) HealthCheck(ctx context.Context) error {
	// Check if service is running
	s.mu.RLock()
	running := s.isRunning
	s.mu.RUnlock()

	if !running {
		return fmt.Errorf("indexer service is not running")
	}

	// Check blockchain client connections
	for chainID, client := range s.blockchainClients {
		if err := client.IsHealthy(ctx); err != nil {
			return fmt.Errorf("blockchain client %s is unhealthy: %w", chainID, err)
		}
	}

	// Check repository connections
	if err := s.checkpointRepo.HealthCheck(ctx); err != nil {
		return fmt.Errorf("checkpoint repository is unhealthy: %w", err)
	}

	return nil
}

// GetIndexingStatus returns the current indexing status for all chains
func (s *IndexerService) GetIndexingStatus(ctx context.Context) (map[string]interface{}, error) {
	status := make(map[string]interface{})

	for chainID, client := range s.blockchainClients {
		chainStatus := make(map[string]interface{})

		// Get checkpoint
		checkpoint, err := s.checkpointRepo.GetCheckpoint(ctx, chainID)
		if err != nil {
			chainStatus["error"] = err.Error()
			status[chainID] = chainStatus
			continue
		}

		// Get latest block
		latestBlock, err := client.GetLatestBlock(ctx)
		if err != nil {
			chainStatus["error"] = err.Error()
			status[chainID] = chainStatus
			continue
		}

		// Calculate lag
		lag := new(big.Int).Sub(latestBlock, checkpoint.LastBlock)

		chainStatus["last_processed_block"] = checkpoint.LastBlock.String()
		chainStatus["latest_block"] = latestBlock.String()
		chainStatus["lag_blocks"] = lag.String()
		chainStatus["last_updated"] = checkpoint.UpdatedAt
		chainStatus["healthy"] = lag.Int64() < 100 // Consider healthy if less than 100 blocks behind

		status[chainID] = chainStatus
	}

	return status, nil
}