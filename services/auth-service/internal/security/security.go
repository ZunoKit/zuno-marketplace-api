package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TokenValidator provides token validation utilities
type TokenValidator struct {
	minTokenLength int
	maxTokenLength int
}

// NewTokenValidator creates a new token validator
func NewTokenValidator() *TokenValidator {
	return &TokenValidator{
		minTokenLength: 32,
		maxTokenLength: 256,
	}
}

// ValidateRefreshToken validates refresh token format and length
func (v *TokenValidator) ValidateRefreshToken(token string) error {
	if token == "" {
		return fmt.Errorf("refresh token is required")
	}

	if len(token) < v.minTokenLength {
		return fmt.Errorf("refresh token is too short")
	}

	if len(token) > v.maxTokenLength {
		return fmt.Errorf("refresh token is too long")
	}

	// Check if token is hex encoded
	if !isHexString(token) {
		return fmt.Errorf("invalid refresh token format")
	}

	return nil
}

// ValidateAccessToken validates access token format
func (v *TokenValidator) ValidateAccessToken(token string) error {
	if token == "" {
		return fmt.Errorf("access token is required")
	}

	// Basic JWT format validation (three parts separated by dots)
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid access token format")
	}

	return nil
}

// AddressValidator provides Ethereum address validation
type AddressValidator struct {
	addressRegex *regexp.Regexp
}

// NewAddressValidator creates a new address validator
func NewAddressValidator() *AddressValidator {
	return &AddressValidator{
		addressRegex: regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`),
	}
}

// ValidateAddress validates Ethereum address format
func (v *AddressValidator) ValidateAddress(address string) error {
	if address == "" {
		return fmt.Errorf("address is required")
	}

	if !v.addressRegex.MatchString(address) {
		return fmt.Errorf("invalid Ethereum address format")
	}

	return nil
}

// NormalizeAddress converts address to lowercase
func (v *AddressValidator) NormalizeAddress(address string) string {
	return strings.ToLower(address)
}

// ChainValidator provides chain ID validation
type ChainValidator struct {
	supportedChains map[string]bool
}

// NewChainValidator creates a new chain validator
func NewChainValidator() *ChainValidator {
	return &ChainValidator{
		supportedChains: map[string]bool{
			"eip155:1":        true, // Ethereum Mainnet
			"eip155:5":        true, // Goerli
			"eip155:11155111": true, // Sepolia
			"eip155:137":      true, // Polygon
			"eip155:80001":    true, // Mumbai
			"eip155:42161":    true, // Arbitrum One
			"eip155:10":       true, // Optimism
			"eip155:8453":     true, // Base
		},
	}
}

// ValidateChainID validates CAIP-2 chain ID format
func (v *ChainValidator) ValidateChainID(chainID string) error {
	if chainID == "" {
		return fmt.Errorf("chain ID is required")
	}

	// Validate CAIP-2 format
	if !regexp.MustCompile(`^[a-z0-9]+:[a-zA-Z0-9]+$`).MatchString(chainID) {
		return fmt.Errorf("invalid CAIP-2 chain ID format")
	}

	// Check if chain is supported
	if !v.supportedChains[chainID] {
		return fmt.Errorf("unsupported chain: %s", chainID)
	}

	return nil
}

// IsSupported checks if a chain is supported
func (v *ChainValidator) IsSupported(chainID string) bool {
	return v.supportedChains[chainID]
}

// DomainValidator provides domain validation
type DomainValidator struct {
	allowedDomains map[string]bool
}

// NewDomainValidator creates a new domain validator
func NewDomainValidator(allowedDomains []string) *DomainValidator {
	domainMap := make(map[string]bool)
	for _, domain := range allowedDomains {
		domainMap[domain] = true
	}

	return &DomainValidator{
		allowedDomains: domainMap,
	}
}

// ValidateDomain validates if domain is allowed
func (v *DomainValidator) ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain is required")
	}

	if len(v.allowedDomains) > 0 && !v.allowedDomains[domain] {
		return fmt.Errorf("domain not allowed: %s", domain)
	}

	return nil
}

// SessionValidator provides session validation
type SessionValidator struct {
	maxSessionAge time.Duration
	maxRefreshAge time.Duration
	requireSecure bool
}

// NewSessionValidator creates a new session validator
func NewSessionValidator() *SessionValidator {
	return &SessionValidator{
		maxSessionAge: 24 * time.Hour,      // Access token max age
		maxRefreshAge: 30 * 24 * time.Hour, // Refresh token max age
		requireSecure: true,                // Require secure connections
	}
}

// ValidateSessionAge validates if session is within acceptable age
func (v *SessionValidator) ValidateSessionAge(createdAt time.Time) error {
	age := time.Since(createdAt)
	if age > v.maxRefreshAge {
		return fmt.Errorf("session has exceeded maximum age")
	}
	return nil
}

// SignatureValidator provides signature validation utilities
type SignatureValidator struct {
	maxMessageSize int
}

// NewSignatureValidator creates a new signature validator
func NewSignatureValidator() *SignatureValidator {
	return &SignatureValidator{
		maxMessageSize: 10 * 1024, // 10KB max message size
	}
}

// ValidateSignature validates signature format
func (v *SignatureValidator) ValidateSignature(signature string) error {
	if signature == "" {
		return fmt.Errorf("signature is required")
	}

	// Remove 0x prefix if present
	sig := strings.TrimPrefix(signature, "0x")

	// Signature should be 65 bytes (130 hex chars)
	if len(sig) != 130 {
		return fmt.Errorf("invalid signature length: expected 130 hex characters, got %d", len(sig))
	}

	// Check if signature is hex encoded
	if !isHexString(sig) {
		return fmt.Errorf("signature must be hex encoded")
	}

	return nil
}

// ValidateMessage validates SIWE message size and format
func (v *SignatureValidator) ValidateMessage(message string) error {
	if message == "" {
		return fmt.Errorf("message is required")
	}

	if len(message) > v.maxMessageSize {
		return fmt.Errorf("message exceeds maximum size of %d bytes", v.maxMessageSize)
	}

	// Basic SIWE message validation
	if !strings.Contains(message, "wants you to sign in") {
		return fmt.Errorf("invalid SIWE message format")
	}

	return nil
}

// Helper functions

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	_, err := hex.DecodeString(s)
	return err == nil
}

// HashRefreshToken creates a secure hash of refresh token for storage
func HashRefreshToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GenerateHMAC generates HMAC for data integrity
func GenerateHMAC(key, data []byte) string {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMAC verifies HMAC for data integrity
func VerifyHMAC(key, data []byte, mac string) bool {
	expectedMAC := GenerateHMAC(key, data)
	return hmac.Equal([]byte(mac), []byte(expectedMAC))
}
