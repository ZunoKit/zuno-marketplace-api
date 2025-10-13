package fixtures

import (
	"crypto/ecdsa"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestWallet represents a test wallet for SIWE authentication
type TestWallet struct {
	privateKey *ecdsa.PrivateKey
	Address    string
	ChainID    uint64
}

// GenerateTestWallet creates a new test wallet
// Returns a cryptographically secure test wallet for Sepolia testnet
func GenerateTestWallet(t *testing.T) *TestWallet {
	t.Helper()

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate test wallet: %v", err)
	}

	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	return &TestWallet{
		privateKey: privateKey,
		Address:    address.Hex(),
		ChainID:    11155111, // Sepolia
	}
}

// SignMessage signs a message with the wallet's private key
// Returns signature as hex string or fails the test
func (w *TestWallet) SignMessage(t *testing.T, message string) string {
	t.Helper()

	// Create hash of the message
	hash := crypto.Keccak256Hash([]byte(message))

	// Sign the hash
	signature, err := crypto.Sign(hash.Bytes(), w.privateKey)
	if err != nil {
		t.Fatalf("failed to sign message: %v", err)
	}

	// Return hex-encoded signature
	return fmt.Sprintf("0x%x", signature)
}

// GetAddress returns the wallet address
func (w *TestWallet) GetAddress() common.Address {
	return common.HexToAddress(w.Address)
}

