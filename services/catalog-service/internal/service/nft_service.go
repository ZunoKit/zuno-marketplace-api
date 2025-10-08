package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
)

// NFTService handles NFT catalog operations
type NFTService struct {
	nftRepo      domain.NFTRepository
	metadataRepo domain.MetadataRepository
	eventRepo    domain.ProcessedEventRepository
	publisher    domain.EventPublisher
	fetcher      domain.MetadataFetcher
}

// NewNFTService creates a new NFT service
func NewNFTService(
	nftRepo domain.NFTRepository,
	metadataRepo domain.MetadataRepository,
	eventRepo domain.ProcessedEventRepository,
	publisher domain.EventPublisher,
	fetcher domain.MetadataFetcher,
) *NFTService {
	return &NFTService{
		nftRepo:      nftRepo,
		metadataRepo: metadataRepo,
		eventRepo:    eventRepo,
		publisher:    publisher,
		fetcher:      fetcher,
	}
}

// ProcessMintEvent processes a mint event from the message queue
func (s *NFTService) ProcessMintEvent(ctx context.Context, message []byte) error {
	// Parse the mint event message
	var event MintEventMessage
	if err := json.Unmarshal(message, &event); err != nil {
		return fmt.Errorf("failed to unmarshal mint event: %w", err)
	}

	// Check if event was already processed (idempotency)
	if processed, err := s.eventRepo.IsProcessed(ctx, event.EventID); err != nil {
		return fmt.Errorf("failed to check if event processed: %w", err)
	} else if processed {
		fmt.Printf("Event %s already processed, skipping\n", event.EventID)
		return nil
	}

	// Process based on token standard
	var nft *domain.NFT
	var err error

	switch event.Standard {
	case "ERC721":
		nft, err = s.processERC721Mint(ctx, &event)
	case "ERC1155":
		nft, err = s.processERC1155Mint(ctx, &event)
	default:
		return fmt.Errorf("unsupported token standard: %s", event.Standard)
	}

	if err != nil {
		return fmt.Errorf("failed to process %s mint: %w", event.Standard, err)
	}

	// Mark event as processed
	if err := s.eventRepo.MarkProcessed(ctx, event.EventID, event.TxHash); err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}

	// Publish domain event for real-time notifications
	if err := s.publishNFTMintedEvent(ctx, nft, &event); err != nil {
		fmt.Printf("Failed to publish NFT minted domain event: %v\n", err)
		// Don't fail the processing, just log the error
	}

	fmt.Printf("Successfully processed mint event for NFT %s:%s\n", nft.Contract, nft.TokenID)
	return nil
}

// processERC721Mint processes an ERC721 mint event
func (s *NFTService) processERC721Mint(ctx context.Context, event *MintEventMessage) (*domain.NFT, error) {
	// Fetch token metadata if available
	metadata, tokenURI, err := s.fetchTokenMetadata(ctx, event.ChainID, event.Contract, event.TokenIDs[0], "ERC721")
	if err != nil {
		fmt.Printf("Failed to fetch metadata for token %s: %v\n", event.TokenIDs[0], err)
		// Continue without metadata
	}

	// Create NFT record
	nft := &domain.NFT{
		ChainID:   event.ChainID,
		Contract:  event.Contract,
		TokenID:   event.TokenIDs[0],
		Owner:     event.To,
		TokenURI:  tokenURI,
		Standard:  "ERC721",
		Supply:    "1", // ERC721 always has supply of 1
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store metadata if fetched
	if metadata != nil {
		metadataDoc, err := s.storeMetadata(ctx, nft, metadata)
		if err != nil {
			fmt.Printf("Failed to store metadata: %v\n", err)
		} else {
			nft.MetadataID = &metadataDoc.ID
		}
	}

	// Upsert NFT record
	if err := s.nftRepo.UpsertNFT(ctx, nft); err != nil {
		return nil, fmt.Errorf("failed to upsert NFT: %w", err)
	}

	return nft, nil
}

// processERC1155Mint processes an ERC1155 mint event
func (s *NFTService) processERC1155Mint(ctx context.Context, event *MintEventMessage) (*domain.NFT, error) {
	// Process each token in the event
	for i, tokenID := range event.TokenIDs {
		amount := "1"
		if i < len(event.Amounts) {
			amount = event.Amounts[i]
		}

		// Fetch token metadata
		metadata, tokenURI, err := s.fetchTokenMetadata(ctx, event.ChainID, event.Contract, tokenID, "ERC1155")
		if err != nil {
			fmt.Printf("Failed to fetch metadata for token %s: %v\n", tokenID, err)
		}

		// Create or update NFT record
		nft := &domain.NFT{
			ChainID:   event.ChainID,
			Contract:  event.Contract,
			TokenID:   tokenID,
			Owner:     event.To, // For ERC1155, this might need balance tracking
			TokenURI:  tokenURI,
			Standard:  "ERC1155",
			Supply:    amount,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Store metadata if fetched
		if metadata != nil {
			metadataDoc, err := s.storeMetadata(ctx, nft, metadata)
			if err != nil {
				fmt.Printf("Failed to store metadata: %v\n", err)
			} else {
				nft.MetadataID = &metadataDoc.ID
			}
		}

		// Upsert NFT record
		if err := s.nftRepo.UpsertNFT(ctx, nft); err != nil {
			fmt.Printf("Failed to upsert NFT %s: %v\n", tokenID, err)
			continue
		}
	}

	// Return the first NFT for the event
	if len(event.TokenIDs) > 0 {
		return s.nftRepo.GetNFT(ctx, event.ChainID, event.Contract, event.TokenIDs[0])
	}

	return nil, nil
}

// fetchTokenMetadata fetches and normalizes token metadata
func (s *NFTService) fetchTokenMetadata(ctx context.Context, chainID, contract, tokenID, standard string) (*domain.NFTMetadata, string, error) {
	// Get token URI from blockchain
	tokenURI, err := s.fetcher.GetTokenURI(ctx, chainID, contract, tokenID, standard)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get token URI: %w", err)
	}

	if tokenURI == "" {
		return nil, "", nil // No metadata available
	}

	// Fetch metadata from URI
	rawMetadata, err := s.fetcher.FetchFromURI(ctx, tokenURI)
	if err != nil {
		return nil, tokenURI, fmt.Errorf("failed to fetch metadata from URI: %w", err)
	}

	// Normalize metadata
	metadata := s.normalizeMetadata(rawMetadata)

	// Process IPFS URIs if present
	metadata.Image = s.processIPFSURI(metadata.Image)
	metadata.AnimationURL = s.processIPFSURI(metadata.AnimationURL)

	return metadata, tokenURI, nil
}

// normalizeMetadata normalizes raw metadata to standard format
func (s *NFTService) normalizeMetadata(raw map[string]interface{}) *domain.NFTMetadata {
	metadata := &domain.NFTMetadata{
		Attributes: []domain.Attribute{},
		Properties: make(map[string]interface{}),
	}

	// Extract standard fields
	if name, ok := raw["name"].(string); ok {
		metadata.Name = name
	}
	if desc, ok := raw["description"].(string); ok {
		metadata.Description = desc
	}
	if image, ok := raw["image"].(string); ok {
		metadata.Image = image
	}
	if animationURL, ok := raw["animation_url"].(string); ok {
		metadata.AnimationURL = animationURL
	}
	if externalURL, ok := raw["external_url"].(string); ok {
		metadata.ExternalURL = externalURL
	}
	if backgroundColor, ok := raw["background_color"].(string); ok {
		metadata.BackgroundColor = backgroundColor
	}

	// Extract attributes (OpenSea standard)
	if attrs, ok := raw["attributes"].([]interface{}); ok {
		for _, attr := range attrs {
			if attrMap, ok := attr.(map[string]interface{}); ok {
				attribute := domain.Attribute{}

				if traitType, ok := attrMap["trait_type"].(string); ok {
					attribute.TraitType = traitType
				}
				if value := attrMap["value"]; value != nil {
					attribute.Value = fmt.Sprintf("%v", value)
				}
				if displayType, ok := attrMap["display_type"].(string); ok {
					attribute.DisplayType = &displayType
				}
				if maxValue := attrMap["max_value"]; maxValue != nil {
					mv := fmt.Sprintf("%v", maxValue)
					attribute.MaxValue = &mv
				}

				metadata.Attributes = append(metadata.Attributes, attribute)
			}
		}
	}

	// Store any additional properties
	for key, value := range raw {
		switch key {
		case "name", "description", "image", "animation_url", "external_url", "background_color", "attributes":
			// Already processed
		default:
			metadata.Properties[key] = value
		}
	}

	return metadata
}

// processIPFSURI converts IPFS URIs to HTTP gateway URLs
func (s *NFTService) processIPFSURI(uri string) string {
	if uri == "" {
		return ""
	}

	// Handle ipfs:// protocol
	if strings.HasPrefix(uri, "ipfs://") {
		hash := strings.TrimPrefix(uri, "ipfs://")
		return fmt.Sprintf("https://ipfs.io/ipfs/%s", hash)
	}

	// Handle /ipfs/ paths
	if strings.HasPrefix(uri, "/ipfs/") {
		return fmt.Sprintf("https://ipfs.io%s", uri)
	}

	// Return as-is if not IPFS
	return uri
}

// storeMetadata stores metadata in the metadata repository
func (s *NFTService) storeMetadata(ctx context.Context, nft *domain.NFT, metadata *domain.NFTMetadata) (*domain.MetadataDocument, error) {
	doc := &domain.MetadataDocument{
		ID:         fmt.Sprintf("%s-%s-%s", nft.ChainID, nft.Contract, nft.TokenID),
		ChainID:    nft.ChainID,
		Contract:   nft.Contract,
		TokenID:    nft.TokenID,
		Normalized: metadata,
		Media: &domain.MediaInfo{
			Image:        metadata.Image,
			AnimationURL: metadata.AnimationURL,
			Processed:    false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Process media URLs
	if doc.Media.Image != "" {
		doc.Media.ImageCDN = s.getCDNURL(doc.Media.Image)
		doc.Media.Processed = true
	}

	if err := s.metadataRepo.UpsertMetadata(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to upsert metadata: %w", err)
	}

	return doc, nil
}

// getCDNURL returns a CDN URL for media (placeholder implementation)
func (s *NFTService) getCDNURL(originalURL string) string {
	// In production, this would upload to CDN and return the CDN URL
	// For now, return the original URL
	return originalURL
}

// publishNFTMintedEvent publishes a domain event for NFT minting
func (s *NFTService) publishNFTMintedEvent(ctx context.Context, nft *domain.NFT, event *MintEventMessage) error {
	domainEvent := map[string]interface{}{
		"schema":     "v1",
		"event_type": "nft.minted",
		"chain_id":   nft.ChainID,
		"contract":   nft.Contract,
		"token_ids":  event.TokenIDs,
		"owner":      nft.Owner,
		"tx_hash":    event.TxHash,
		"standard":   nft.Standard,
		"metadata": map[string]interface{}{
			"name":        "",
			"description": "",
			"image":       "",
		},
	}

	// Include metadata if available
	if nft.MetadataID != nil {
		metadata, err := s.metadataRepo.GetMetadata(ctx, *nft.MetadataID)
		if err == nil && metadata != nil && metadata.Normalized != nil {
			domainEvent["metadata"] = map[string]interface{}{
				"name":        metadata.Normalized.Name,
				"description": metadata.Normalized.Description,
				"image":       metadata.Normalized.Image,
			}
		}
	}

	// Publish to mints.domain topic
	routingKey := fmt.Sprintf("upserted.%s.%s.%s",
		strings.ReplaceAll(nft.ChainID, ":", "-"),
		nft.Contract,
		nft.TokenID,
	)

	return s.publisher.Publish(ctx, "mints.domain", routingKey, domainEvent)
}

// GetNFT retrieves an NFT from the catalog
func (s *NFTService) GetNFT(ctx context.Context, chainID, contract, tokenID string) (*domain.NFT, error) {
	nft, err := s.nftRepo.GetNFT(ctx, chainID, contract, tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get NFT: %w", err)
	}

	// Load metadata if available
	if nft.MetadataID != nil {
		metadata, err := s.metadataRepo.GetMetadata(ctx, *nft.MetadataID)
		if err == nil && metadata != nil {
			nft.Metadata = metadata.Normalized
		}
	}

	return nft, nil
}

// ListNFTs lists NFTs with pagination
func (s *NFTService) ListNFTs(ctx context.Context, filter *domain.NFTFilter) ([]*domain.NFT, error) {
	return s.nftRepo.ListNFTs(ctx, filter)
}

// GetNFTsByOwner retrieves all NFTs owned by a specific address
func (s *NFTService) GetNFTsByOwner(ctx context.Context, chainID, owner string, limit, offset int) ([]*domain.NFT, error) {
	filter := &domain.NFTFilter{
		ChainID: &chainID,
		Owner:   &owner,
		Limit:   limit,
		Offset:  offset,
	}

	return s.nftRepo.ListNFTs(ctx, filter)
}

// GetCollectionNFTs retrieves all NFTs in a collection
func (s *NFTService) GetCollectionNFTs(ctx context.Context, chainID, contract string, limit, offset int) ([]*domain.NFT, error) {
	filter := &domain.NFTFilter{
		ChainID:  &chainID,
		Contract: &contract,
		Limit:    limit,
		Offset:   offset,
	}

	return s.nftRepo.ListNFTs(ctx, filter)
}

// MintEventMessage represents a mint event message from the queue
type MintEventMessage struct {
	Schema    string   `json:"schema"`
	EventID   string   `json:"event_id"`
	Standard  string   `json:"standard"`
	ChainID   string   `json:"chain_id"`
	Contract  string   `json:"contract"`
	To        string   `json:"to"`
	TokenIDs  []string `json:"token_ids"`
	Amounts   []string `json:"amounts"`
	TxHash    string   `json:"tx_hash"`
	BlockNum  string   `json:"block_num"`
	Timestamp int64    `json:"timestamp"`
}
