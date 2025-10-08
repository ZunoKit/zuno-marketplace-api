package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/infrastructure/blockchain"
)

// MintIndexer handles NFT minting event indexing
type MintIndexer struct {
	service *IndexerService
}

// NewMintIndexer creates a new mint indexer
func NewMintIndexer(service *IndexerService) *MintIndexer {
	return &MintIndexer{
		service: service,
	}
}

// ProcessMintEvents processes mint-related events for a specific chain and collection
func (mi *MintIndexer) ProcessMintEvents(ctx context.Context, chainID string, collectionAddress string, fromBlock, toBlock *big.Int) error {
	client, exists := mi.service.blockchainClients[chainID]
	if !exists {
		return fmt.Errorf("blockchain client not found for chain %s", chainID)
	}

	// Process ERC721 Transfer events (from zero address = mint)
	if err := mi.processERC721MintEvents(ctx, chainID, collectionAddress, fromBlock, toBlock, client); err != nil {
		fmt.Printf("Error processing ERC721 mint events: %v\n", err)
		// Continue processing other events
	}

	// Process ERC1155 TransferSingle events (from zero address = mint)
	if err := mi.processERC1155SingleMintEvents(ctx, chainID, collectionAddress, fromBlock, toBlock, client); err != nil {
		fmt.Printf("Error processing ERC1155 single mint events: %v\n", err)
		// Continue processing other events
	}

	// Process ERC1155 TransferBatch events (from zero address = mint)
	if err := mi.processERC1155BatchMintEvents(ctx, chainID, collectionAddress, fromBlock, toBlock, client); err != nil {
		fmt.Printf("Error processing ERC1155 batch mint events: %v\n", err)
		// Continue processing other events
	}

	return nil
}

// processERC721MintEvents processes ERC721 Transfer events where from=0x0
func (mi *MintIndexer) processERC721MintEvents(ctx context.Context, chainID, collectionAddress string, fromBlock, toBlock *big.Int, client *blockchain.Client) error {
	// Filter for Transfer events with from=zero address (mints)
	zeroAddress := "0x" + strings.Repeat("0", 64) // 32 bytes of zeros
	filter := &domain.LogFilter{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []string{collectionAddress},
		Topics: []string{
			domain.TransferSignature,
			zeroAddress, // from = zero address indicates mint
		},
	}

	logs, err := client.GetLogs(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to get ERC721 mint logs: %w", err)
	}

	for _, log := range logs {
		if err := mi.processERC721MintLog(ctx, chainID, log, client); err != nil {
			fmt.Printf("Failed to process ERC721 mint log %s:%d: %v\n", log.TxHash, log.LogIndex, err)
			continue
		}
	}

	return nil
}

// processERC721MintLog processes a single ERC721 mint (Transfer from 0x0)
func (mi *MintIndexer) processERC721MintLog(ctx context.Context, chainID string, log *domain.Log, client *blockchain.Client) error {
	// Check confirmations
	confirmations, err := client.GetConfirmations(ctx, log.BlockNumber)
	if err != nil {
		return fmt.Errorf("failed to get confirmations: %w", err)
	}

	// Parse the Transfer event
	transferEvent, err := mi.parseERC721Transfer(log)
	if err != nil {
		return fmt.Errorf("failed to parse ERC721 Transfer: %w", err)
	}

	// Create raw event
	rawEvent := &domain.RawEvent{
		ChainID:         chainID,
		TxHash:          log.TxHash,
		LogIndex:        log.LogIndex,
		BlockNumber:     log.BlockNumber,
		BlockHash:       log.BlockHash,
		ContractAddress: log.Address,
		EventName:       "Transfer",
		EventSignature:  domain.TransferSignature,
		RawData: map[string]interface{}{
			"topics": log.Topics,
			"data":   log.Data,
		},
		Confirmations: confirmations,
		ObservedAt:    time.Now(),
	}

	// Serialize parsed data
	parsedJSON, err := json.Marshal(transferEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal parsed event: %w", err)
	}
	rawEvent.ParsedJSON = string(parsedJSON)

	// Store raw event in MongoDB
	if err := mi.service.eventRepo.StoreRawEvent(ctx, rawEvent); err != nil {
		return fmt.Errorf("failed to store raw event: %w", err)
	}

	// Publish if confirmed
	requiredConfirmations := mi.service.getRequiredConfirmations(chainID)
	if confirmations >= requiredConfirmations {
		// Create mint event for publishing
		mintEvent := &MintEvent{
			Standard:  "ERC721",
			ChainID:   chainID,
			Contract:  log.Address,
			To:        transferEvent.To,
			TokenID:   transferEvent.TokenID,
			Amount:    "1", // ERC721 always mints 1
			TxHash:    log.TxHash,
			BlockNum:  log.BlockNumber.String(),
			Timestamp: time.Now().Unix(),
		}

		if err := mi.publishMintEvent(ctx, chainID, mintEvent); err != nil {
			return fmt.Errorf("failed to publish mint event: %w", err)
		}
		fmt.Printf("Published ERC721 mint event for token %s to %s\n", transferEvent.TokenID, transferEvent.To)
	}

	return nil
}

// processERC1155SingleMintEvents processes ERC1155 TransferSingle events where from=0x0
func (mi *MintIndexer) processERC1155SingleMintEvents(ctx context.Context, chainID, collectionAddress string, fromBlock, toBlock *big.Int, client *blockchain.Client) error {
	// Filter for TransferSingle events
	filter := &domain.LogFilter{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []string{collectionAddress},
		Topics:    []string{domain.TransferSingleSignature},
	}

	logs, err := client.GetLogs(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to get ERC1155 single mint logs: %w", err)
	}

	for _, log := range logs {
		// Parse to check if from=0x0
		event, err := mi.parseERC1155TransferSingle(log)
		if err != nil {
			fmt.Printf("Failed to parse ERC1155 TransferSingle: %v\n", err)
			continue
		}

		// Check if it's a mint (from = 0x0)
		if !mi.isZeroAddress(event.From) {
			continue
		}

		if err := mi.processERC1155SingleMintLog(ctx, chainID, log, event, client); err != nil {
			fmt.Printf("Failed to process ERC1155 single mint log %s:%d: %v\n", log.TxHash, log.LogIndex, err)
			continue
		}
	}

	return nil
}

// processERC1155SingleMintLog processes a single ERC1155 mint (TransferSingle from 0x0)
func (mi *MintIndexer) processERC1155SingleMintLog(ctx context.Context, chainID string, log *domain.Log, event *domain.TransferSingleEvent, client *blockchain.Client) error {
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
		EventName:       "TransferSingle",
		EventSignature:  domain.TransferSingleSignature,
		RawData: map[string]interface{}{
			"topics": log.Topics,
			"data":   log.Data,
		},
		Confirmations: confirmations,
		ObservedAt:    time.Now(),
	}

	// Serialize parsed data
	parsedJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal parsed event: %w", err)
	}
	rawEvent.ParsedJSON = string(parsedJSON)

	// Store raw event
	if err := mi.service.eventRepo.StoreRawEvent(ctx, rawEvent); err != nil {
		return fmt.Errorf("failed to store raw event: %w", err)
	}

	// Publish if confirmed
	requiredConfirmations := mi.service.getRequiredConfirmations(chainID)
	if confirmations >= requiredConfirmations {
		// Create mint event for publishing
		mintEvent := &MintEvent{
			Standard:  "ERC1155",
			ChainID:   chainID,
			Contract:  log.Address,
			To:        event.To,
			TokenID:   event.ID,
			Amount:    event.Value,
			TxHash:    log.TxHash,
			BlockNum:  log.BlockNumber.String(),
			Timestamp: time.Now().Unix(),
		}

		if err := mi.publishMintEvent(ctx, chainID, mintEvent); err != nil {
			return fmt.Errorf("failed to publish mint event: %w", err)
		}
		fmt.Printf("Published ERC1155 mint event for token %s (amount: %s) to %s\n", event.ID, event.Value, event.To)
	}

	return nil
}

// processERC1155BatchMintEvents processes ERC1155 TransferBatch events where from=0x0
func (mi *MintIndexer) processERC1155BatchMintEvents(ctx context.Context, chainID, collectionAddress string, fromBlock, toBlock *big.Int, client *blockchain.Client) error {
	// Filter for TransferBatch events
	filter := &domain.LogFilter{
		FromBlock: fromBlock,
		ToBlock:   toBlock,
		Addresses: []string{collectionAddress},
		Topics:    []string{domain.TransferBatchSignature},
	}

	logs, err := client.GetLogs(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to get ERC1155 batch mint logs: %w", err)
	}

	for _, log := range logs {
		// Parse to check if from=0x0
		event, err := mi.parseERC1155TransferBatch(log)
		if err != nil {
			fmt.Printf("Failed to parse ERC1155 TransferBatch: %v\n", err)
			continue
		}

		// Check if it's a mint (from = 0x0)
		if !mi.isZeroAddress(event.From) {
			continue
		}

		if err := mi.processERC1155BatchMintLog(ctx, chainID, log, event, client); err != nil {
			fmt.Printf("Failed to process ERC1155 batch mint log %s:%d: %v\n", log.TxHash, log.LogIndex, err)
			continue
		}
	}

	return nil
}

// processERC1155BatchMintLog processes a batch ERC1155 mint (TransferBatch from 0x0)
func (mi *MintIndexer) processERC1155BatchMintLog(ctx context.Context, chainID string, log *domain.Log, event *domain.TransferBatchEvent, client *blockchain.Client) error {
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
		EventName:       "TransferBatch",
		EventSignature:  domain.TransferBatchSignature,
		RawData: map[string]interface{}{
			"topics": log.Topics,
			"data":   log.Data,
		},
		Confirmations: confirmations,
		ObservedAt:    time.Now(),
	}

	// Serialize parsed data
	parsedJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal parsed event: %w", err)
	}
	rawEvent.ParsedJSON = string(parsedJSON)

	// Store raw event
	if err := mi.service.eventRepo.StoreRawEvent(ctx, rawEvent); err != nil {
		return fmt.Errorf("failed to store raw event: %w", err)
	}

	// Publish if confirmed
	requiredConfirmations := mi.service.getRequiredConfirmations(chainID)
	if confirmations >= requiredConfirmations {
		// Create mint events for each token in the batch
		for i, tokenID := range event.IDs {
			mintEvent := &MintEvent{
				Standard:   "ERC1155",
				ChainID:    chainID,
				Contract:   log.Address,
				To:         event.To,
				TokenID:    tokenID,
				Amount:     event.Values[i],
				TxHash:     log.TxHash,
				BlockNum:   log.BlockNumber.String(),
				Timestamp:  time.Now().Unix(),
				BatchIndex: &i, // Track position in batch
			}

			if err := mi.publishMintEvent(ctx, chainID, mintEvent); err != nil {
				fmt.Printf("Failed to publish batch mint event for token %s: %v\n", tokenID, err)
				continue
			}
		}
		fmt.Printf("Published ERC1155 batch mint events for %d tokens to %s\n", len(event.IDs), event.To)
	}

	return nil
}

// Helper functions

func (mi *MintIndexer) parseERC721Transfer(log *domain.Log) (*domain.TransferEvent, error) {
	if len(log.Topics) < 4 {
		return nil, fmt.Errorf("invalid ERC721 Transfer event topics")
	}

	return &domain.TransferEvent{
		From:    mi.topicToAddress(log.Topics[1]),
		To:      mi.topicToAddress(log.Topics[2]),
		TokenID: mi.topicToUint256(log.Topics[3]),
	}, nil
}

func (mi *MintIndexer) parseERC1155TransferSingle(log *domain.Log) (*domain.TransferSingleEvent, error) {
	if len(log.Topics) < 4 {
		return nil, fmt.Errorf("invalid ERC1155 TransferSingle event topics")
	}

	// Decode data field for id and value
	// This would need proper ABI decoding in production
	// For now, using simplified parsing
	return &domain.TransferSingleEvent{
		Operator: mi.topicToAddress(log.Topics[1]),
		From:     mi.topicToAddress(log.Topics[2]),
		To:       mi.topicToAddress(log.Topics[3]),
		ID:       "0", // Would be decoded from log.Data
		Value:    "1", // Would be decoded from log.Data
	}, nil
}

func (mi *MintIndexer) parseERC1155TransferBatch(log *domain.Log) (*domain.TransferBatchEvent, error) {
	if len(log.Topics) < 4 {
		return nil, fmt.Errorf("invalid ERC1155 TransferBatch event topics")
	}

	// Decode data field for ids and values arrays
	// This would need proper ABI decoding in production
	return &domain.TransferBatchEvent{
		Operator: mi.topicToAddress(log.Topics[1]),
		From:     mi.topicToAddress(log.Topics[2]),
		To:       mi.topicToAddress(log.Topics[3]),
		IDs:      []string{"0"}, // Would be decoded from log.Data
		Values:   []string{"1"}, // Would be decoded from log.Data
	}, nil
}

func (mi *MintIndexer) topicToAddress(topic string) string {
	// Remove 0x prefix and leading zeros
	addr := strings.TrimPrefix(topic, "0x")
	if len(addr) > 40 {
		addr = addr[len(addr)-40:]
	}
	return "0x" + addr
}

func (mi *MintIndexer) topicToUint256(topic string) string {
	// Convert hex string to decimal
	topic = strings.TrimPrefix(topic, "0x")
	n := new(big.Int)
	n.SetString(topic, 16)
	return n.String()
}

func (mi *MintIndexer) isZeroAddress(addr string) bool {
	addr = strings.ToLower(strings.TrimPrefix(addr, "0x"))
	return addr == "" || addr == "0" || addr == strings.Repeat("0", 40)
}

// MintEvent represents a normalized mint event for publishing
type MintEvent struct {
	Standard   string `json:"standard"` // ERC721 or ERC1155
	ChainID    string `json:"chain_id"`
	Contract   string `json:"contract"`
	To         string `json:"to"`
	TokenID    string `json:"token_id"`
	Amount     string `json:"amount"`
	TxHash     string `json:"tx_hash"`
	BlockNum   string `json:"block_num"`
	Timestamp  int64  `json:"timestamp"`
	BatchIndex *int   `json:"batch_index,omitempty"` // For batch mints
}

// publishMintEvent publishes a mint event to RabbitMQ
func (mi *MintIndexer) publishMintEvent(ctx context.Context, chainID string, event *MintEvent) error {
	// Publish to RabbitMQ with routing key: minted.eip155-{chainId}
	routingKey := fmt.Sprintf("minted.eip155-%s", strings.TrimPrefix(chainID, "eip155-"))

	message := map[string]interface{}{
		"schema":    "v1",
		"event_id":  fmt.Sprintf("%s-%s-%s", event.TxHash, event.Contract, event.TokenID),
		"standard":  event.Standard,
		"chain_id":  chainID,
		"contract":  event.Contract,
		"to":        event.To,
		"token_ids": []string{event.TokenID},
		"amounts":   []string{event.Amount},
		"tx_hash":   event.TxHash,
		"block_num": event.BlockNum,
		"timestamp": event.Timestamp,
	}

	if event.BatchIndex != nil {
		message["batch_index"] = *event.BatchIndex
	}

	return mi.service.publisher.PublishMintEvent(ctx, "mints.events", routingKey, message)
}
