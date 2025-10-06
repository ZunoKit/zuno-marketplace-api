package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/orchestrator-service/internal/domain"
)

// ValidateCreateCollectionInput validates the create collection input
func ValidateCreateCollectionInput(in domain.PrepareCreateCollectionInput) error {
	if in.ChainID == "" {
		return fmt.Errorf("chain ID is required")
	}
	if in.Name == "" {
		return fmt.Errorf("collection name is required")
	}
	if in.Symbol == "" {
		return fmt.Errorf("collection symbol is required")
	}
	if in.Creator == "" {
		return fmt.Errorf("creator address is required")
	}
	if in.Type == "" {
		return fmt.Errorf("collection type is required")
	}

	if in.Type != domain.StdERC721 && in.Type != domain.StdERC1155 {
		return fmt.Errorf("unsupported collection type: %s. Supported types: %s, %s", in.Type, domain.StdERC721, domain.StdERC1155)
	}

	if len(in.Name) > 100 {
		return fmt.Errorf("collection name too long (max 100 characters)")
	}
	if len(in.Name) < 1 {
		return fmt.Errorf("collection name cannot be empty")
	}

	if len(in.Symbol) > 10 {
		return fmt.Errorf("collection symbol too long (max 10 characters)")
	}
	if len(in.Symbol) < 1 {
		return fmt.Errorf("collection symbol cannot be empty")
	}

	if !IsValidEthereumAddress(in.Creator) {
		return fmt.Errorf("invalid creator address format")
	}

	if !IsValidCAIP2ChainID(in.ChainID) {
		return fmt.Errorf("invalid chain ID format (expected CAIP-2 format like 'eip155:1')")
	}

	if in.MintPrice != nil && *in.MintPrice > 0 {
	}
	if in.RoyaltyFee != nil && *in.RoyaltyFee > 10000 {
		return fmt.Errorf("royalty fee cannot exceed 100%%")
	}
	if in.MaxSupply != nil && *in.MaxSupply == 0 {
		return fmt.Errorf("max supply must be greater than 0")
	}
	if in.MaxSupply != nil && *in.MaxSupply > 1000000 {
		return fmt.Errorf("max supply too high (max 1,000,000)")
	}

	if in.MintLimitPerWallet != nil && *in.MintLimitPerWallet == 0 {
		return fmt.Errorf("mint limit per wallet must be greater than 0")
	}

	if in.MintStartTime != nil && *in.MintStartTime > 0 {
		if *in.MintStartTime < uint64(time.Now().Unix()) {
			return fmt.Errorf("mint start time cannot be in the past")
		}
	}

	return nil
}

// IsValidEthereumAddress validates if a string is a valid Ethereum address
func IsValidEthereumAddress(address string) bool {
	if len(address) != 42 {
		return false
	}
	if !strings.HasPrefix(address, "0x") {
		return false
	}
	for _, char := range address[2:] {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}

// IsValidCAIP2ChainID validates if a string is a valid CAIP-2 chain ID
func IsValidCAIP2ChainID(chainID string) bool {
	parts := strings.Split(chainID, ":")
	if len(parts) != 2 {
		return false
	}
	namespace := parts[0]
	reference := parts[1]
	return len(namespace) > 0 && len(reference) > 0
}
