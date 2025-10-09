package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/getsentry/sentry-go"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/infrastructure/blockchain"
)

// MintIndexer handles NFT minting event indexing
type MintIndexer struct {
	service    *IndexerService
	erc1155ABI abi.ABI
}

// ERC1155 ABI for TransferBatch event
const ERC1155ABI = `[{
	"anonymous": false,
	"inputs": [
		{"indexed": true, "name": "operator", "type": "address"},
		{"indexed": true, "name": "from", "type": "address"},
		{"indexed": true, "name": "to", "type": "address"},
		{"indexed": false, "name": "ids", "type": "uint256[]"},
		{"indexed": false, "name": "values", "type": "uint256[]"}
	],
	"name": "TransferBatch",
	"type": "event"
}]`

// NewMintIndexer creates a new mint indexer
func NewMintIndexer(service *IndexerService) *MintIndexer {
	// Parse ERC1155 ABI
	contractABI, err := abi.JSON(strings.NewReader(ERC1155ABI))
	if err != nil {
		// Log error but don't fail initialization
		sentry.CaptureException(fmt.Errorf("failed to parse ERC1155 ABI: %w", err))
	}

	return &MintIndexer{
		service:    service,
		erc1155ABI: contractABI,
	}
}

// ProcessMintEvents processes mint-related events for a specific chain and collection
func (mi *MintIndexer) ProcessMintEvents(ctx context.Context, chainID string, collectionAddress string, fromBlock, toBlock *big.Int) error {
	// Start Sentry transaction for performance monitoring
	span := sentry.StartSpan(ctx, "indexer.process_mint_events")
	span.SetTag("chain_id", chainID)
	span.SetTag("collection", collectionAddress)
	span.SetTag("from_block", fromBlock.String())
	span.SetTag("to_block", toBlock.String())
	defer span.Finish()
	ctx = span.Context()

	client, exists := mi.service.blockchainClients[chainID]
	if !exists {
		err := fmt.Errorf("blockchain client not found for chain %s", chainID)
		sentry.CaptureException(err)
		return err
	}

	// Process ERC721 Transfer events (from zero address = mint)
	if err := mi.processERC721MintEvents(ctx, chainID, collectionAddress, fromBlock, toBlock, client); err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("event_type", "ERC721_Transfer")
			scope.SetTag("chain_id", chainID)
			scope.SetTag("collection", collectionAddress)
			scope.SetLevel(sentry.LevelWarning)
			sentry.CaptureException(err)
		})
		fmt.Printf("Error processing ERC721 mint events: %v\n", err)
		// Continue processing other events
	}

	// Process ERC1155 TransferSingle events (from zero address = mint)
	if err := mi.processERC1155SingleMintEvents(ctx, chainID, collectionAddress, fromBlock, toBlock, client); err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("event_type", "ERC1155_TransferSingle")
			scope.SetTag("chain_id", chainID)
			scope.SetTag("collection", collectionAddress)
			scope.SetLevel(sentry.LevelWarning)
			sentry.CaptureException(err)
		})
		fmt.Printf("Error processing ERC1155 single mint events: %v\n", err)
		// Continue processing other events
	}

	// Process ERC1155 TransferBatch events (from zero address = mint)
	if err := mi.processERC1155BatchMintEvents(ctx, chainID, collectionAddress, fromBlock, toBlock, client); err != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("event_type", "ERC1155_TransferBatch")
			scope.SetTag("chain_id", chainID)
			scope.SetTag("collection", collectionAddress)
			scope.SetLevel(sentry.LevelWarning)
			sentry.CaptureException(err)
		})
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
		event, err := mi.ParseERC1155TransferSingle(log)
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
		event, err := mi.ParseERC1155TransferBatch(log)
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

// ParseERC1155TransferSingle parses an ERC1155 TransferSingle event from a log entry
func (mi *MintIndexer) ParseERC1155TransferSingle(log *domain.Log) (*domain.TransferSingleEvent, error) {
	if len(log.Topics) < 4 {
		return nil, fmt.Errorf("invalid ERC1155 TransferSingle event topics")
	}

	// Topics contain indexed parameters
	operator := mi.topicToAddress(log.Topics[1])
	from := mi.topicToAddress(log.Topics[2])
	to := mi.topicToAddress(log.Topics[3])

	// Decode data field for id and value (non-indexed parameters)
	data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failed to decode log data: %w", err)
	}

	// ERC1155 TransferSingle data contains:
	// - bytes 0-32: uint256 id
	// - bytes 32-64: uint256 value
	if len(data) < 64 {
		return nil, fmt.Errorf("invalid data length for TransferSingle: expected at least 64 bytes, got %d", len(data))
	}

	// Parse ID (first 32 bytes)
	id := new(big.Int).SetBytes(data[0:32])

	// Parse Value (second 32 bytes)
	value := new(big.Int).SetBytes(data[32:64])

	return &domain.TransferSingleEvent{
		Operator: operator,
		From:     from,
		To:       to,
		ID:       id.String(),
		Value:    value.String(),
	}, nil
}

// ParseERC1155TransferBatch parses an ERC1155 TransferBatch event from a log entry
func (mi *MintIndexer) ParseERC1155TransferBatch(log *domain.Log) (*domain.TransferBatchEvent, error) {
	if len(log.Topics) < 4 {
		return nil, fmt.Errorf("invalid ERC1155 TransferBatch event topics")
	}

	// Topics contain indexed parameters
	operator := mi.topicToAddress(log.Topics[1])
	from := mi.topicToAddress(log.Topics[2])
	to := mi.topicToAddress(log.Topics[3])

	// Decode data field for ids and values arrays (non-indexed parameters)
	data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
	if err != nil {
		return nil, fmt.Errorf("failed to decode log data: %w", err)
	}

	// Use proper ABI unpacking for dynamic arrays
	// First, check if ABI was properly loaded
	if len(mi.erc1155ABI.Events) == 0 {
		// Fallback to manual parsing if ABI is not loaded
		return mi.parseTransferBatchManually(data, operator, from, to)
	}

	// Create a map for unpacking the non-indexed parameters
	unpacked := make(map[string]interface{})

	event, ok := mi.erc1155ABI.Events["TransferBatch"]
	if !ok {
		// Fallback to manual parsing
		return mi.parseTransferBatchManually(data, operator, from, to)
	}

	// Get non-indexed inputs only (ids and values)
	nonIndexedInputs := make(abi.Arguments, 0)
	for _, input := range event.Inputs {
		if !input.Indexed {
			nonIndexedInputs = append(nonIndexedInputs, input)
		}
	}

	// Unpack the data
	err = nonIndexedInputs.UnpackIntoMap(unpacked, data)
	if err != nil {
		// Fallback to manual parsing if ABI unpacking fails
		return mi.parseTransferBatchManually(data, operator, from, to)
	}

	// Extract arrays from unpacked data
	idsInterface, ok := unpacked["ids"]
	if !ok {
		return nil, fmt.Errorf("ids not found in unpacked data")
	}

	valuesInterface, ok := unpacked["values"]
	if !ok {
		return nil, fmt.Errorf("values not found in unpacked data")
	}

	// Convert to []*big.Int
	idsArray, ok := idsInterface.([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to cast ids to []*big.Int")
	}

	valuesArray, ok := valuesInterface.([]*big.Int)
	if !ok {
		return nil, fmt.Errorf("failed to cast values to []*big.Int")
	}

	// Convert to string arrays
	ids := make([]string, len(idsArray))
	values := make([]string, len(valuesArray))

	for i, id := range idsArray {
		ids[i] = id.String()
	}
	for i, val := range valuesArray {
		values[i] = val.String()
	}

	return &domain.TransferBatchEvent{
		Operator: operator,
		From:     from,
		To:       to,
		IDs:      ids,
		Values:   values,
	}, nil
}

// parseTransferBatchManually manually parses TransferBatch event data without ABI
func (mi *MintIndexer) parseTransferBatchManually(data []byte, operator, from, to string) (*domain.TransferBatchEvent, error) {
	if len(data) < 128 {
		return nil, fmt.Errorf("invalid data length for TransferBatch: expected at least 128 bytes, got %d", len(data))
	}

	// Read offsets to arrays
	idsOffset := new(big.Int).SetBytes(data[0:32]).Uint64()
	valuesOffset := new(big.Int).SetBytes(data[32:64]).Uint64()

	// Read ids array
	if idsOffset >= uint64(len(data)) {
		return nil, fmt.Errorf("invalid ids offset: %d", idsOffset)
	}

	idsLengthStart := idsOffset
	if idsLengthStart+32 > uint64(len(data)) {
		return nil, fmt.Errorf("invalid ids array length position")
	}
	idsLength := new(big.Int).SetBytes(data[idsLengthStart : idsLengthStart+32]).Uint64()

	ids := make([]string, idsLength)
	for i := uint64(0); i < idsLength; i++ {
		start := idsOffset + 32 + (i * 32)
		end := start + 32
		if end > uint64(len(data)) {
			return nil, fmt.Errorf("invalid ids array data")
		}
		ids[i] = new(big.Int).SetBytes(data[start:end]).String()
	}

	// Read values array
	if valuesOffset >= uint64(len(data)) {
		return nil, fmt.Errorf("invalid values offset: %d", valuesOffset)
	}

	valuesLengthStart := valuesOffset
	if valuesLengthStart+32 > uint64(len(data)) {
		return nil, fmt.Errorf("invalid values array length position")
	}
	valuesLength := new(big.Int).SetBytes(data[valuesLengthStart : valuesLengthStart+32]).Uint64()

	values := make([]string, valuesLength)
	for i := uint64(0); i < valuesLength; i++ {
		start := valuesOffset + 32 + (i * 32)
		end := start + 32
		if end > uint64(len(data)) {
			return nil, fmt.Errorf("invalid values array data")
		}
		values[i] = new(big.Int).SetBytes(data[start:end]).String()
	}

	return &domain.TransferBatchEvent{
		Operator: operator,
		From:     from,
		To:       to,
		IDs:      ids,
		Values:   values,
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

// decodeERC1155BatchData decodes the data field of TransferBatch event
func (mi *MintIndexer) decodeERC1155BatchData(data []byte) ([]string, []string, error) {
	if len(data) < 64 {
		return nil, nil, fmt.Errorf("data too short for batch transfer")
	}

	// The data contains two dynamic arrays: uint256[] ids and uint256[] values
	// Layout:
	// 0x00-0x20: offset to ids array
	// 0x20-0x40: offset to values array
	// ids array data...
	// values array data...

	// Read offsets
	idsOffset := new(big.Int).SetBytes(data[0:32]).Uint64()
	valuesOffset := new(big.Int).SetBytes(data[32:64]).Uint64()

	// Parse ids array
	ids, err := mi.parseDynamicUint256Array(data, idsOffset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse ids array: %w", err)
	}

	// Parse values array
	values, err := mi.parseDynamicUint256Array(data, valuesOffset)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse values array: %w", err)
	}

	if len(ids) != len(values) {
		return nil, nil, fmt.Errorf("ids and values arrays have different lengths: %d vs %d", len(ids), len(values))
	}

	return ids, values, nil
}

// parseDynamicUint256Array parses a dynamic uint256[] from data at given offset
func (mi *MintIndexer) parseDynamicUint256Array(data []byte, offset uint64) ([]string, error) {
	if offset+32 > uint64(len(data)) {
		return nil, fmt.Errorf("offset out of bounds")
	}

	// Read array length
	length := new(big.Int).SetBytes(data[offset : offset+32]).Uint64()

	// Read array elements
	result := make([]string, length)
	for i := uint64(0); i < length; i++ {
		elemOffset := offset + 32 + (i * 32)
		if elemOffset+32 > uint64(len(data)) {
			return nil, fmt.Errorf("element offset out of bounds")
		}
		value := new(big.Int).SetBytes(data[elemOffset : elemOffset+32])
		result[i] = value.String()
	}

	return result, nil
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

	// Publish as a generic collection event with mint routing
	publishableEvent := &domain.PublishableEvent{
		EventType: "mint.indexed",
		Data:      message,
	}
	return mi.service.publisher.PublishCollectionEvent(ctx, event.ChainID, publishableEvent)
}
