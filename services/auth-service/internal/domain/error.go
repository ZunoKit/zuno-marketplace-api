package domain

import "errors"

var (
	ErrNonceNotFound       = errors.New("Nonce not found")
	ErrNonceUsed           = errors.New("Nonce used")
	ErrNonceExpired        = errors.New("Nonce expired")
	ErrNonceInvalid        = errors.New("Nonce invalid")
	ErrNonceAlreadyUsed    = errors.New("Nonce already used")
	ErrNonceAlreadyExpired = errors.New("Nonce already expired")
	ErrNonceAlreadyInvalid = errors.New("Nonce already invalid")
	ErrInvalidAccountID    = errors.New("Invalid account ID")
	ErrInvalidChainID      = errors.New("Invalid chain ID")
)
