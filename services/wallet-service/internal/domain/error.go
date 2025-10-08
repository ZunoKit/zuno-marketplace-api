package domain

import "errors"

var (
	ErrWalletNotFound      = errors.New("wallet not found")
	ErrInvalidAddress      = errors.New("invalid wallet address")
	ErrInvalidChainID      = errors.New("invalid chain ID")
	ErrInvalidUserID       = errors.New("invalid user ID")
	ErrWalletAlreadyLinked = errors.New("wallet already linked to user")
	ErrCannotRemovePrimary = errors.New("cannot remove primary wallet")
	ErrNoWalletsFound      = errors.New("no wallets found for user")
)
