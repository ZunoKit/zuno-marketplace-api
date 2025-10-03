package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/wallet-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type Service struct {
	repo domain.WalletRepository
}

func NewWalletService(repo domain.WalletRepository) domain.WalletService {
	return &Service{
		repo: repo,
	}
}

func (s *Service) UpsertLink(ctx context.Context, link domain.WalletLink) (*domain.WalletUpsertResult, error) {
	if err := s.validateWalletLink(link); err != nil {
		return nil, err
	}
	link.Address = normalizeAddress(link.Address)
	link.ChainID = normalizeChainID(link.ChainID)

	var result *domain.WalletUpsertResult
	err := s.repo.WithTx(ctx, func(tx domain.TxWalletRepository) error {
		if err := tx.AcquireAccountLock(ctx, link.AccountID); err != nil {
			return fmt.Errorf("lock account: %w", err)
		}
		if err := tx.AcquireAddressLock(ctx, link.ChainID, link.Address); err != nil {
			return fmt.Errorf("lock address: %w", err)
		}

		addrRow, err := tx.GetByAddressTx(ctx, link.ChainID, link.Address)
		if err != nil && err != domain.ErrWalletNotFound {
			return err
		}

		accRow, err := tx.GetByAccountIDTx(ctx, link.AccountID)
		if err != nil && err != domain.ErrWalletNotFound {
			return err
		}

		switch {
		case addrRow != nil && accRow != nil:
			// Cùng bản ghi?
			if addrRow.ID != accRow.ID {
				// địa chỉ đã gắn user khác hoặc data lệch
				return domain.ErrUnauthorizedAccess
			}
			// Không ép unset primary nếu request không promote
			var setPrimary *bool
			primaryChanged := false
			if link.IsPrimary && !accRow.IsPrimary {
				t := true
				setPrimary = &t
				if err := tx.DemoteOtherPrimariesTx(ctx, accRow.UserID, accRow.ChainID, accRow.ID); err == nil {
					primaryChanged = true
				}
			}
			now := time.Now()
			// chỉ cập nhật last_seen/verified
			updated, err := tx.UpdateWalletMetaTx(ctx, accRow.ID, setPrimary, nil, &now, nil)
			if err != nil {
				updated = accRow
			}
			result = &domain.WalletUpsertResult{Link: updated, Created: false, PrimaryChanged: primaryChanged}
			return nil

		case addrRow != nil && accRow == nil:
			// Địa chỉ đã có
			if addrRow.UserID != link.UserID {
				return domain.ErrUnauthorizedAccess
			}
			// Account mới trỏ tới cùng địa chỉ? chính sách thường là KHÔNG cho,
			// hoặc update account_id (nếu cho phép) -> cần thêm repo UpdateAccountIDTx (không khuyến nghị).
			return fmt.Errorf("address already linked to another account for this user")

		case addrRow == nil && accRow != nil:
			// Account đã tồn tại nhưng địa chỉ mới -> UPDATE địa chỉ (không insert)
			if accRow.UserID != link.UserID {
				return domain.ErrUnauthorizedAccess
			}
			// Đảm bảo địa chỉ mới chưa thuộc user khác
			if _, err := tx.GetByAddressTx(ctx, link.ChainID, link.Address); err == nil {
				return fmt.Errorf("address already linked to another user")
			} else if err != domain.ErrWalletNotFound {
				return err
			}

			updated, uerr := tx.UpdateWalletAddressTx(ctx, accRow.ID, link.ChainID, link.Address)
			if uerr != nil {
				return uerr
			}

			primaryChanged := false
			if link.IsPrimary && !updated.IsPrimary {
				t := true
				if _, u2 := tx.UpdateWalletMetaTx(ctx, updated.ID, &t, nil, nil, nil); u2 == nil {
					_ = tx.DemoteOtherPrimariesTx(ctx, updated.UserID, updated.ChainID, updated.ID)
					primaryChanged = true
				}
			}
			result = &domain.WalletUpsertResult{Link: updated, Created: false, PrimaryChanged: primaryChanged}
			return nil

		default:
			// Tạo mới
			primaryChanged := false
			if link.IsPrimary {
				_ = tx.DemoteOtherPrimariesTx(ctx, link.UserID, link.ChainID, "")
				primaryChanged = true
			} else {
				if _, err := tx.GetPrimaryByUserChainTx(ctx, link.UserID, link.ChainID); err == domain.ErrWalletNotFound {
					link.IsPrimary = true
					primaryChanged = true
				}
			}
			inserted, ierr := tx.InsertWalletTx(ctx, link)
			if ierr != nil {
				return ierr
			}
			result = &domain.WalletUpsertResult{Link: inserted, Created: true, PrimaryChanged: primaryChanged}
			return nil
		}
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) validateWalletLink(link domain.WalletLink) error {
	if link.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	if link.AccountID == "" {
		return fmt.Errorf("account_id is required")
	}

	if link.Address == "" {
		return fmt.Errorf("address is required")
	}

	if link.ChainID == "" {
		return fmt.Errorf("chain_id is required")
	}

	// Validate address format (basic Ethereum address validation)
	if !isValidEthereumAddress(link.Address) {
		return domain.ErrInvalidAddress
	}

	// Validate chain ID format (CAIP-2)
	if !isValidChainID(link.ChainID) {
		return domain.ErrInvalidChainID
	}

	return nil
}

// Helper functions
func normalizeAddress(addr string) string {
	return redis.NormalizeAddress(addr)
}

func normalizeChainID(chainID string) string {
	return redis.NormalizeChainID(chainID)
}

func isValidEthereumAddress(addr string) bool {
	if len(addr) != 42 {
		return false
	}

	if !strings.HasPrefix(addr, "0x") {
		return false
	}

	// Check if all characters after 0x are valid hex
	for _, char := range addr[2:] {
		if !((char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F')) {
			return false
		}
	}

	return true
}

func isValidChainID(chainID string) bool {
	// CAIP-2 format: namespace:reference
	// e.g., "eip155:1", "eip155:137"
	parts := strings.Split(chainID, ":")
	if len(parts) != 2 {
		return false
	}

	namespace := parts[0]
	reference := parts[1]

	// Validate namespace (should be alphanumeric)
	if namespace == "" || reference == "" {
		return false
	}

	// Basic validation - can be extended based on requirements
	return len(namespace) > 0 && len(reference) > 0
}

// Exported wrappers used in tests
func (s *Service) ValidateWalletLink(link domain.WalletLink) error { return s.validateWalletLink(link) }
func NormalizeAddress(addr string) string                          { return normalizeAddress(addr) }
func NormalizeChainID(chainID string) string                       { return normalizeChainID(chainID) }
func IsValidEthereumAddress(addr string) bool                      { return isValidEthereumAddress(addr) }
func IsValidChainID(chainID string) bool                           { return isValidChainID(chainID) }
