package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/sha3"
)

// Event signatures (keccak256 hashes of event signatures)
var (
	// CollectionCreated(address indexed collectionAddress, address indexed creator, string name, string symbol, uint256 timestamp)
	CollectionCreatedSignature = EventSignature("CollectionCreated(address,address,string,string,uint256)")

	// CollectionMinted(address indexed collection, address indexed to, uint256 indexed tokenId, uint256 amount)
	CollectionMintedSignature = EventSignature("CollectionMinted(address,address,uint256,uint256)")

	// Transfer(address indexed from, address indexed to, uint256 indexed tokenId)
	TransferSignature = EventSignature("Transfer(address,address,uint256)")

	// TransferSingle(address indexed operator, address indexed from, address indexed to, uint256 id, uint256 value)
	TransferSingleSignature = EventSignature("TransferSingle(address,address,address,uint256,uint256)")

	// TransferBatch(address indexed operator, address indexed from, address indexed to, uint256[] ids, uint256[] values)
	TransferBatchSignature = EventSignature("TransferBatch(address,address,address,uint256[],uint256[])")

	// Approval(address indexed owner, address indexed approved, uint256 indexed tokenId)
	ApprovalSignature = EventSignature("Approval(address,address,uint256)")

	// ApprovalForAll(address indexed owner, address indexed operator, bool approved)
	ApprovalForAllSignature = EventSignature("ApprovalForAll(address,address,bool)")

	// RoyaltySet(address indexed collection, address indexed recipient, uint256 percentage)
	RoyaltySetSignature = EventSignature("RoyaltySet(address,address,uint256)")

	// PriceUpdate(address indexed collection, uint256 oldPrice, uint256 newPrice)
	PriceUpdateSignature = EventSignature("PriceUpdate(address,uint256,uint256)")

	// CollectionPaused(address indexed collection)
	CollectionPausedSignature = EventSignature("CollectionPaused(address)")

	// CollectionUnpaused(address indexed collection)
	CollectionUnpausedSignature = EventSignature("CollectionUnpaused(address)")

	// OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
	OwnershipTransferredSignature = EventSignature("OwnershipTransferred(address,address)")

	// URI(string value, uint256 indexed id)
	URISignature = EventSignature("URI(string,uint256)")

	// ContractURIUpdated(string newURI)
	ContractURIUpdatedSignature = EventSignature("ContractURIUpdated(string)")

	// Sale(address indexed buyer, address indexed seller, uint256 indexed tokenId, uint256 price)
	SaleSignature = EventSignature("Sale(address,address,uint256,uint256)")

	// ListingCreated(address indexed seller, address indexed collection, uint256 indexed tokenId, uint256 price)
	ListingCreatedSignature = EventSignature("ListingCreated(address,address,uint256,uint256)")

	// ListingCancelled(address indexed seller, address indexed collection, uint256 indexed tokenId)
	ListingCancelledSignature = EventSignature("ListingCancelled(address,address,uint256)")

	// OfferCreated(address indexed buyer, address indexed collection, uint256 indexed tokenId, uint256 price, uint256 expiry)
	OfferCreatedSignature = EventSignature("OfferCreated(address,address,uint256,uint256,uint256)")

	// OfferAccepted(address indexed seller, address indexed buyer, address indexed collection, uint256 tokenId, uint256 price)
	OfferAcceptedSignature = EventSignature("OfferAccepted(address,address,address,uint256,uint256)")

	// OfferCancelled(address indexed buyer, address indexed collection, uint256 indexed tokenId)
	OfferCancelledSignature = EventSignature("OfferCancelled(address,address,uint256)")
)

// EventSignature calculates the keccak256 hash of an event signature
func EventSignature(sig string) string {
	// Remove spaces
	sig = strings.ReplaceAll(sig, " ", "")

	// Calculate Keccak256 hash
	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(sig))
	return "0x" + hex.EncodeToString(hash.Sum(nil))
}

// EventSignatureMap maps event names to their signatures
var EventSignatureMap = map[string]string{
	"CollectionCreated":    CollectionCreatedSignature,
	"CollectionMinted":     CollectionMintedSignature,
	"Transfer":             TransferSignature,
	"TransferSingle":       TransferSingleSignature,
	"TransferBatch":        TransferBatchSignature,
	"Approval":             ApprovalSignature,
	"ApprovalForAll":       ApprovalForAllSignature,
	"RoyaltySet":           RoyaltySetSignature,
	"PriceUpdate":          PriceUpdateSignature,
	"CollectionPaused":     CollectionPausedSignature,
	"CollectionUnpaused":   CollectionUnpausedSignature,
	"OwnershipTransferred": OwnershipTransferredSignature,
	"URI":                  URISignature,
	"ContractURIUpdated":   ContractURIUpdatedSignature,
	"Sale":                 SaleSignature,
	"ListingCreated":       ListingCreatedSignature,
	"ListingCancelled":     ListingCancelledSignature,
	"OfferCreated":         OfferCreatedSignature,
	"OfferAccepted":        OfferAcceptedSignature,
	"OfferCancelled":       OfferCancelledSignature,
}

// GetEventName returns the event name for a given signature
func GetEventName(signature string) string {
	for name, sig := range EventSignatureMap {
		if sig == signature {
			return name
		}
	}
	return "Unknown"
}

// IsKnownEvent checks if an event signature is known
func IsKnownEvent(signature string) bool {
	for _, sig := range EventSignatureMap {
		if sig == signature {
			return true
		}
	}
	return false
}

// Event data structures for parsed events

// CollectionCreatedEvent represents a parsed CollectionCreated event
type CollectionCreatedEvent struct {
	CollectionAddress string `json:"collection_address"`
	Creator           string `json:"creator"`
	Name              string `json:"name"`
	Symbol            string `json:"symbol"`
	Timestamp         uint64 `json:"timestamp"`
}

// CollectionMintedEvent represents a parsed CollectionMinted event
type CollectionMintedEvent struct {
	Collection string `json:"collection"`
	To         string `json:"to"`
	TokenID    string `json:"token_id"`
	Amount     string `json:"amount"`
}

// TransferEvent represents a parsed Transfer event
type TransferEvent struct {
	From    string `json:"from"`
	To      string `json:"to"`
	TokenID string `json:"token_id"`
}

// TransferSingleEvent represents a parsed TransferSingle event (ERC1155)
type TransferSingleEvent struct {
	Operator string `json:"operator"`
	From     string `json:"from"`
	To       string `json:"to"`
	ID       string `json:"id"`
	Value    string `json:"value"`
}

// TransferBatchEvent represents a parsed TransferBatch event (ERC1155)
type TransferBatchEvent struct {
	Operator string   `json:"operator"`
	From     string   `json:"from"`
	To       string   `json:"to"`
	IDs      []string `json:"ids"`
	Values   []string `json:"values"`
}

// ApprovalEvent represents a parsed Approval event
type ApprovalEvent struct {
	Owner    string `json:"owner"`
	Approved string `json:"approved"`
	TokenID  string `json:"token_id"`
}

// ApprovalForAllEvent represents a parsed ApprovalForAll event
type ApprovalForAllEvent struct {
	Owner    string `json:"owner"`
	Operator string `json:"operator"`
	Approved bool   `json:"approved"`
}

// SaleEvent represents a parsed Sale event
type SaleEvent struct {
	Buyer   string `json:"buyer"`
	Seller  string `json:"seller"`
	TokenID string `json:"token_id"`
	Price   string `json:"price"`
}

// ListingCreatedEvent represents a parsed ListingCreated event
type ListingCreatedEvent struct {
	Seller     string `json:"seller"`
	Collection string `json:"collection"`
	TokenID    string `json:"token_id"`
	Price      string `json:"price"`
}

// ListingCancelledEvent represents a parsed ListingCancelled event
type ListingCancelledEvent struct {
	Seller     string `json:"seller"`
	Collection string `json:"collection"`
	TokenID    string `json:"token_id"`
}

// OfferCreatedEvent represents a parsed OfferCreated event
type OfferCreatedEvent struct {
	Buyer      string `json:"buyer"`
	Collection string `json:"collection"`
	TokenID    string `json:"token_id"`
	Price      string `json:"price"`
	Expiry     uint64 `json:"expiry"`
}

// OfferAcceptedEvent represents a parsed OfferAccepted event
type OfferAcceptedEvent struct {
	Seller     string `json:"seller"`
	Buyer      string `json:"buyer"`
	Collection string `json:"collection"`
	TokenID    string `json:"token_id"`
	Price      string `json:"price"`
}

// OfferCancelledEvent represents a parsed OfferCancelled event
type OfferCancelledEvent struct {
	Buyer      string `json:"buyer"`
	Collection string `json:"collection"`
	TokenID    string `json:"token_id"`
}
