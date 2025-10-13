package fixtures

import (
	"fmt"
	"time"
)

// GenerateSIWEMessage creates a Sign-In with Ethereum message
func GenerateSIWEMessage(address, nonce string) string {
	return fmt.Sprintf(`example.com wants you to sign in with your Ethereum account:
%s

Sign in to NFT Marketplace

URI: https://example.com
Version: 1
Chain ID: 11155111
Nonce: %s
Issued At: %s`, address, nonce, time.Now().Format(time.RFC3339))
}

