package service

import (
	"context"
	"errors"
	"strings"

	"github.com/quangdang46/NFT-Marketplace/services/user-service/internal/domain"
)

type Service struct {
	userRepo domain.UserRepository
}

func NewUserService(userRepo domain.UserRepository) domain.UserService {
	return &Service{
		userRepo: userRepo,
	}
}

func (s *Service) EnsureUser(ctx context.Context, accountID domain.AccountID, address domain.Address, chainID domain.ChainID) (*domain.EnsureUserResult, error) {
	// Validate
	if err := domain.ValidateAccountID(accountID); err != nil {
		return nil, err
	}
	if err := domain.ValidateAddress(address); err != nil {
		return nil, err
	}
	if err := domain.ValidateChainID(chainID); err != nil {
		return nil, err
	}

	acc := string(accountID)
	addr := strings.ToLower(string(address))
	ch := string(chainID)

	// Fast path: đã có mapping thì trả luôn (không TX)
	if uid, err := s.userRepo.GetUserIDByAccount(ctx, acc); err == nil {
		return &domain.EnsureUserResult{UserID: uid, Created: false}, nil
	} else if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}

	var out domain.EnsureUserResult
	// Slow path: TX + advisory lock theo accountID để chống race
	if err := s.userRepo.WithTx(ctx, func(tx domain.TxUserRepository) error {
		// lock theo accountID
		if err := tx.AcquireAccountLock(ctx, acc); err != nil {
			return err
		}

		// double-check bên trong TX
		if uid, err := tx.GetUserIDByAccountTx(ctx, acc); err == nil {
			out.UserID = uid
			out.Created = false
			_ = tx.TouchUserAccountTx(ctx, acc, addr) // cập nhật last_seen_at/address (best effort)
			return nil
		} else if !errors.Is(err, domain.ErrUserNotFound) {
			return err
		}

		// tạo user + profile
		uid, err := tx.CreateUserTx(ctx)
		if err != nil {
			return err
		}
		if err := tx.CreateProfileTx(ctx, uid); err != nil {
			// best effort, không fail toàn bộ
		}

		// map account -> user (unique theo account_id)
		if err := tx.UpsertUserAccountTx(ctx, uid, acc, addr, ch); err != nil {
			// nếu conflict hiếm gặp, đọc lại
			if uid2, err2 := tx.GetUserIDByAccountTx(ctx, acc); err2 == nil {
				out.UserID = uid2
				out.Created = false
				return nil
			}
			return err
		}

		out.UserID = uid
		out.Created = true
		return nil
	}); err != nil {
		return nil, err
	}

	return &out, nil

}
