package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/indexer-service/internal/domain"
)

const (
	// SafeBlockDepth is the number of blocks to wait before considering a block safe from reorg
	SafeBlockDepth = 64
	// MaxReorgDepth is the maximum depth we'll handle for reorgs
	MaxReorgDepth = 128
)

// ReorgHandler handles blockchain reorganizations
type ReorgHandler struct {
	service *IndexerService
}

// NewReorgHandler creates a new reorganization handler
func NewReorgHandler(service *IndexerService) *ReorgHandler {
	return &ReorgHandler{
		service: service,
	}
}

// CheckAndHandleReorg checks for chain reorganization and handles it if detected
func (rh *ReorgHandler) CheckAndHandleReorg(ctx context.Context, chainID string, currentBlock *domain.Block) (bool, error) {
	// Get last checkpoint
	checkpoint, err := rh.service.repo.GetCheckpoint(ctx, chainID)
	if err != nil {
		// No checkpoint yet, this is the first block
		return false, nil
	}

	// Check if the previous block hash matches
	if checkpoint.LastBlockHash != nil && currentBlock.ParentHash != *checkpoint.LastBlockHash {
		// Reorg detected!
		return rh.handleReorg(ctx, chainID, checkpoint, currentBlock)
	}

	// No reorg, update checkpoint with continuity
	return false, rh.updateCheckpointWithContinuity(ctx, chainID, checkpoint, currentBlock)
}

// handleReorg handles a detected reorganization
func (rh *ReorgHandler) handleReorg(ctx context.Context, chainID string, checkpoint *domain.IndexerCheckpoint, currentBlock *domain.Block) (bool, error) {
	fmt.Printf("REORG DETECTED on chain %s at block %s\n", chainID, currentBlock.Number)

	// Find the common ancestor block
	commonAncestor, err := rh.findCommonAncestor(ctx, chainID, checkpoint.LastBlock, currentBlock)
	if err != nil {
		return false, fmt.Errorf("failed to find common ancestor: %w", err)
	}

	// Calculate affected blocks
	affectedBlocks := checkpoint.LastBlock.Uint64() - commonAncestor.Uint64()
	if affectedBlocks > MaxReorgDepth {
		return false, fmt.Errorf("reorg depth %d exceeds maximum %d", affectedBlocks, MaxReorgDepth)
	}

	// Record reorg in history
	reorgHistory := &domain.ReorgHistory{
		ChainID:        chainID,
		DetectedAt:     time.Now(),
		ForkBlock:      currentBlock.Number.Uint64(),
		OldChainHead:   checkpoint.LastBlock.Uint64(),
		NewChainHead:   currentBlock.Number.Uint64(),
		OldBlockHash:   *checkpoint.LastBlockHash,
		NewBlockHash:   currentBlock.Hash,
		AffectedBlocks: int(affectedBlocks),
		RollbackTo:     commonAncestor.Uint64(),
	}

	// Get affected data (NFTs, collections) for rollback
	affectedData, err := rh.getAffectedData(ctx, chainID, commonAncestor, checkpoint.LastBlock)
	if err != nil {
		fmt.Printf("Warning: Failed to get affected data: %v\n", err)
	} else {
		dataJSON, _ := json.Marshal(affectedData)
		reorgHistory.DataAffected = string(dataJSON)
	}

	// Save reorg history
	if err := rh.service.repo.SaveReorgHistory(ctx, reorgHistory); err != nil {
		fmt.Printf("Warning: Failed to save reorg history: %v\n", err)
	}

	// Rollback to common ancestor
	if err := rh.rollbackToBlock(ctx, chainID, commonAncestor); err != nil {
		return false, fmt.Errorf("failed to rollback: %w", err)
	}

	// Update checkpoint to common ancestor
	checkpoint.LastBlock = commonAncestor
	checkpoint.LastBlockHash = nil // Will be updated when we re-process
	checkpoint.ReorgDetectedCount++
	checkpoint.LastReorgAt = &reorgHistory.DetectedAt

	if err := rh.service.repo.UpdateCheckpoint(ctx, checkpoint); err != nil {
		return false, fmt.Errorf("failed to update checkpoint after reorg: %w", err)
	}

	// Notify about reorg (could publish event to message queue)
	rh.notifyReorg(ctx, chainID, reorgHistory)

	return true, nil
}

// findCommonAncestor finds the common ancestor block between two chains
func (rh *ReorgHandler) findCommonAncestor(ctx context.Context, chainID string, lastBlock *big.Int, currentBlock *domain.Block) (*big.Int, error) {
	// Binary search for common ancestor
	left := new(big.Int).Sub(lastBlock, big.NewInt(MaxReorgDepth))
	if left.Sign() < 0 {
		left = big.NewInt(0)
	}
	right := new(big.Int).Set(lastBlock)

	for left.Cmp(right) < 0 {
		mid := new(big.Int).Add(left, right)
		mid.Div(mid, big.NewInt(2))

		// Get block at mid height from blockchain
		blockAtMid, err := rh.service.getBlockByNumber(ctx, chainID, mid)
		if err != nil {
			return nil, fmt.Errorf("failed to get block at height %s: %w", mid.String(), err)
		}

		// Check if this block exists in our database with same hash
		exists, err := rh.service.repo.BlockExistsWithHash(ctx, chainID, mid, blockAtMid.Hash)
		if err != nil {
			return nil, err
		}

		if exists {
			// This block is common, try higher
			left = new(big.Int).Add(mid, big.NewInt(1))
		} else {
			// This block differs, try lower
			right = mid
		}
	}

	// Go back one more block to be safe
	if left.Sign() > 0 {
		left.Sub(left, big.NewInt(1))
	}

	return left, nil
}

// getAffectedData gets data that will be affected by the rollback
func (rh *ReorgHandler) getAffectedData(ctx context.Context, chainID string, fromBlock, toBlock *big.Int) (map[string]interface{}, error) {
	affectedData := make(map[string]interface{})

	// Get affected NFT mints
	mints, err := rh.service.repo.GetMintsInBlockRange(ctx, chainID, fromBlock, toBlock)
	if err != nil {
		return nil, err
	}
	affectedData["affected_mints"] = len(mints)
	affectedData["mint_ids"] = mints

	// Get affected collections
	collections, err := rh.service.repo.GetCollectionsInBlockRange(ctx, chainID, fromBlock, toBlock)
	if err != nil {
		return nil, err
	}
	affectedData["affected_collections"] = len(collections)
	affectedData["collection_addresses"] = collections

	// Get affected transactions
	txCount, err := rh.service.repo.CountTransactionsInBlockRange(ctx, chainID, fromBlock, toBlock)
	if err != nil {
		return nil, err
	}
	affectedData["affected_transactions"] = txCount

	return affectedData, nil
}

// rollbackToBlock rolls back all data to a specific block
func (rh *ReorgHandler) rollbackToBlock(ctx context.Context, chainID string, blockNumber *big.Int) error {
	// Mark NFTs minted after this block as "reorged"
	if err := rh.service.repo.MarkNFTsAsReorged(ctx, chainID, blockNumber); err != nil {
		return fmt.Errorf("failed to mark NFTs as reorged: %w", err)
	}

	// Mark collections created after this block as "reorged"
	if err := rh.service.repo.MarkCollectionsAsReorged(ctx, chainID, blockNumber); err != nil {
		return fmt.Errorf("failed to mark collections as reorged: %w", err)
	}

	// Delete events after this block
	if err := rh.service.repo.DeleteEventsAfterBlock(ctx, chainID, blockNumber); err != nil {
		return fmt.Errorf("failed to delete events: %w", err)
	}

	return nil
}

// updateCheckpointWithContinuity updates checkpoint maintaining block continuity
func (rh *ReorgHandler) updateCheckpointWithContinuity(ctx context.Context, chainID string, checkpoint *domain.IndexerCheckpoint, currentBlock *domain.Block) error {
	// Update checkpoint with new block info
	checkpoint.LastBlock = currentBlock.Number
	checkpoint.LastBlockHash = &currentBlock.Hash
	checkpoint.PreviousBlockHash = &currentBlock.ParentHash

	// Update safe block (64 blocks behind)
	if currentBlock.Number.Cmp(big.NewInt(SafeBlockDepth)) > 0 {
		safeBlockNum := new(big.Int).Sub(currentBlock.Number, big.NewInt(SafeBlockDepth))
		safeBlock, err := rh.service.getBlockByNumber(ctx, chainID, safeBlockNum)
		if err == nil {
			checkpoint.SafeBlock = safeBlockNum
			checkpoint.SafeBlockHash = &safeBlock.Hash
		}
	}

	return rh.service.repo.UpdateCheckpoint(ctx, checkpoint)
}

// notifyReorg notifies about chain reorganization
func (rh *ReorgHandler) notifyReorg(ctx context.Context, chainID string, reorg *domain.ReorgHistory) {
	// Log the reorg
	fmt.Printf("Chain Reorg Notification:\n")
	fmt.Printf("  Chain: %s\n", chainID)
	fmt.Printf("  Fork at block: %d\n", reorg.ForkBlock)
	fmt.Printf("  Rolled back from: %d to %d\n", reorg.OldChainHead, reorg.RollbackTo)
	fmt.Printf("  Affected blocks: %d\n", reorg.AffectedBlocks)

	// In production, you would:
	// 1. Send notification to monitoring system (e.g., Sentry)
	// 2. Publish event to message queue for other services
	// 3. Send alerts to administrators
	// 4. Update dashboards/metrics
}

// ValidateBlockContinuity validates that blocks are continuous
func (rh *ReorgHandler) ValidateBlockContinuity(ctx context.Context, chainID string, blocks []*domain.Block) error {
	if len(blocks) < 2 {
		return nil
	}

	for i := 1; i < len(blocks); i++ {
		prevBlock := blocks[i-1]
		currBlock := blocks[i]

		// Check block numbers are sequential
		expectedNum := new(big.Int).Add(prevBlock.Number, big.NewInt(1))
		if currBlock.Number.Cmp(expectedNum) != 0 {
			return fmt.Errorf("non-sequential blocks: %s -> %s", prevBlock.Number, currBlock.Number)
		}

		// Check parent hash matches
		if currBlock.ParentHash != prevBlock.Hash {
			return fmt.Errorf("parent hash mismatch at block %s", currBlock.Number)
		}
	}

	return nil
}
