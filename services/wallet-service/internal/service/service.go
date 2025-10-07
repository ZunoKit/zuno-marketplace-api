package service

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
)

type Service struct {
	walletRepo domain.WalletRepository
	publisher  domain.WalletEventPublisher
}

func NewWalletService(walletRepo domain.WalletRepository, publisher domain.WalletEventPublisher) domain.WalletService {
	return &Service{
		walletRepo: walletRepo,
		publisher:  publisher,
	}
}

func (s *Service) UpsertLink(ctx context.Context, userID domain.UserID, accountID string, address domain.Address, chainID domain.ChainID, isPrimary bool, walletType, connector, label string) (*domain.WalletLink, bool, bool, error) {
	// Validate inputs
	if err := s.validateUpsertLinkInputs(userID, accountID, address, chainID); err != nil {
		return nil, false, false, err
	}

	// Normalize address to lowercase
	address = domain.Address(strings.ToLower(string(address)))

	// Check if link already exists
	existingLink, err := s.walletRepo.GetLinkByUserAndAddress(ctx, userID, address)
	if err == nil && existingLink != nil {
		// Link already exists, check if we need to update primary status
		primaryChanged := false
		if isPrimary && !existingLink.IsPrimary {
			// Update to primary
			if err := s.walletRepo.UpdatePrimaryWallet(ctx, userID, existingLink.ID); err != nil {
				return nil, false, false, fmt.Errorf("failed to update primary wallet: %w", err)
			}
			existingLink.IsPrimary = true
			primaryChanged = true
		}

		// Update metadata if provided
		if walletType != "" {
			existingLink.Type = domain.WalletType(walletType)
		}
		if connector != "" {
			existingLink.Connector = connector
		}
		if label != "" {
			existingLink.Label = label
		}
		existingLink.UpdatedAt = time.Now()

		if err := s.walletRepo.UpdateLink(ctx, existingLink); err != nil {
			log.Printf("Warning: Failed to update wallet metadata: %v", err)
		}

		return existingLink, false, primaryChanged, nil
	}

	// Create new wallet link
	now := time.Now()
	walletID := uuid.New().String()

	// Determine wallet type if not specified
	if walletType == "" {
		walletType = "eoa" // Default to EOA
	}

	// Check if this should be the primary wallet (first wallet for user)
	userWallets, _ := s.walletRepo.GetUserWallets(ctx, userID)
	if len(userWallets) == 0 {
		isPrimary = true // First wallet is always primary
	}

	link := &domain.WalletLink{
		ID:         domain.WalletID(walletID),
		UserID:     userID,
		AccountID:  accountID,
		Address:    address,
		ChainID:    chainID,
		IsPrimary:  isPrimary,
		Type:       domain.WalletType(walletType),
		Connector:  connector,
		Label:      label,
		VerifiedAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// Create the link
	if err := s.walletRepo.CreateLink(ctx, link); err != nil {
		return nil, false, false, fmt.Errorf("failed to create wallet link: %w", err)
	}

	// If this is set as primary, update other wallets
	if isPrimary && len(userWallets) > 0 {
		if err := s.walletRepo.UpdatePrimaryWallet(ctx, userID, link.ID); err != nil {
			log.Printf("Warning: Failed to update primary wallet status: %v", err)
		}
	}

	// Publish wallet linked event (non-blocking)
	if s.publisher != nil {
		go func() {
			event := &domain.WalletLinkedEvent{
				UserID:         userID,
				WalletID:       link.ID,
				Address:        address,
				ChainID:        chainID,
				IsPrimary:      isPrimary,
				PrimaryChanged: isPrimary && len(userWallets) > 0,
				LinkedAt:       now,
			}
			if err := s.publisher.PublishWalletLinked(context.Background(), event); err != nil {
				log.Printf("Failed to publish wallet linked event: %v", err)
			}
		}()
	}

	log.Printf("Created wallet link %s for user %s, address %s", walletID, userID, address)
	return link, true, false, nil
}

func (s *Service) GetUserWallets(ctx context.Context, userID domain.UserID) ([]*domain.WalletLink, error) {
	// Validate user ID
	if userID == "" {
		return nil, domain.ErrInvalidUserID
	}

	wallets, err := s.walletRepo.GetUserWallets(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user wallets: %w", err)
	}

	if len(wallets) == 0 {
		return []*domain.WalletLink{}, nil // Return empty slice instead of nil
	}

	return wallets, nil
}

func (s *Service) GetWalletByAddress(ctx context.Context, address domain.Address) (*domain.WalletLink, error) {
	// Validate and normalize address
	if address == "" {
		return nil, domain.ErrInvalidAddress
	}

	address = domain.Address(strings.ToLower(string(address)))
	if !isValidEthereumAddress(string(address)) {
		return nil, domain.ErrInvalidAddress
	}

	wallet, err := s.walletRepo.GetWalletByAddress(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet by address: %w", err)
	}

	return wallet, nil
}

func (s *Service) SetPrimaryWallet(ctx context.Context, userID domain.UserID, walletID domain.WalletID) error {
	// Validate inputs
	if userID == "" {
		return domain.ErrInvalidUserID
	}
	if walletID == "" {
		return fmt.Errorf("wallet ID is required")
	}

	// Verify wallet belongs to user
	wallet, err := s.walletRepo.GetLink(ctx, walletID)
	if err != nil {
		return domain.ErrWalletNotFound
	}

	if wallet.UserID != userID {
		return fmt.Errorf("wallet does not belong to user")
	}

	// Update primary wallet
	if err := s.walletRepo.UpdatePrimaryWallet(ctx, userID, walletID); err != nil {
		return fmt.Errorf("failed to set primary wallet: %w", err)
	}

	log.Printf("Set wallet %s as primary for user %s", walletID, userID)
	return nil
}

func (s *Service) RemoveWallet(ctx context.Context, userID domain.UserID, walletID domain.WalletID) error {
	// Validate inputs
	if userID == "" {
		return domain.ErrInvalidUserID
	}
	if walletID == "" {
		return fmt.Errorf("wallet ID is required")
	}

	// Get wallet to verify ownership and check if primary
	wallet, err := s.walletRepo.GetLink(ctx, walletID)
	if err != nil {
		return domain.ErrWalletNotFound
	}

	if wallet.UserID != userID {
		return fmt.Errorf("wallet does not belong to user")
	}

	// Don't allow removing primary wallet if it's the only one
	if wallet.IsPrimary {
		userWallets, err := s.walletRepo.GetUserWallets(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to check user wallets: %w", err)
		}

		if len(userWallets) <= 1 {
			return domain.ErrCannotRemovePrimary
		}

		// Set another wallet as primary before removing
		for _, w := range userWallets {
			if w.ID != walletID {
				if err := s.walletRepo.UpdatePrimaryWallet(ctx, userID, w.ID); err != nil {
					return fmt.Errorf("failed to update primary wallet: %w", err)
				}
				break
			}
		}
	}

	// Remove the wallet
	if err := s.walletRepo.DeleteLink(ctx, walletID); err != nil {
		return fmt.Errorf("failed to remove wallet: %w", err)
	}

	log.Printf("Removed wallet %s for user %s", walletID, userID)
	return nil
}

// validateUpsertLinkInputs validates the input parameters for UpsertLink
func (s *Service) validateUpsertLinkInputs(userID domain.UserID, accountID string, address domain.Address, chainID domain.ChainID) error {
	if userID == "" {
		return domain.ErrInvalidUserID
	}

	if accountID == "" {
		return fmt.Errorf("account ID is required")
	}

	if address == "" {
		return domain.ErrInvalidAddress
	}

	// Validate Ethereum address format
	address = domain.Address(strings.ToLower(string(address)))
	if !isValidEthereumAddress(string(address)) {
		return domain.ErrInvalidAddress
	}

	// Validate CAIP-2 chain ID format
	if chainID == "" {
		return domain.ErrInvalidChainID
	}

	if !isValidCAIP2ChainID(string(chainID)) {
		return domain.ErrInvalidChainID
	}

	return nil
}

// isValidEthereumAddress validates Ethereum address format
func isValidEthereumAddress(address string) bool {
	matched, _ := regexp.MatchString(`^0x[0-9a-f]{40}$`, address)
	return matched
}

// isValidCAIP2ChainID validates CAIP-2 chain ID format
func isValidCAIP2ChainID(chainID string) bool {
	matched, _ := regexp.MatchString(`^[a-z0-9]+:[a-zA-Z0-9]+$`, chainID)
	return matched
}
