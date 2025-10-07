package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/utils"
)

type Service struct {
	repository domain.MediaRepository
	pinner     domain.Pinner
	storage    domain.Storage
}

func NewMediaService(
	repository domain.MediaRepository,
	pinner domain.Pinner,
	storage domain.Storage,
) domain.MediaService {
	return &Service{
		repository: repository,
		pinner:     pinner,
		storage:    storage,
	}
}

func (s *Service) UploadAndPin(ctx context.Context, meta domain.UploadMeta, r io.Reader, sizeHint int64) (asset *domain.AssetDoc, dedup bool, err error) {
	// Validate inputs
	if r == nil {
		return nil, false, fmt.Errorf("reader cannot be nil")
	}
	if meta.Filename == "" {
		return nil, false, fmt.Errorf("filename is required")
	}
	if meta.Mime == "" {
		return nil, false, fmt.Errorf("mime type is required")
	}

	// Read the entire content to calculate SHA256 and prepare for pinning
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read content: %w", err)
	}

	// Calculate SHA256 hash for deduplication
	hash := sha256.Sum256(content)
	sha256Hash := hex.EncodeToString(hash[:])

	// Create asset document
	assetID := uuid.New().String()
	asset = &domain.AssetDoc{
		ID:          assetID,
		Kind:        meta.Kind,
		Mime:        meta.Mime,
		Bytes:       int64(len(content)),
		Width:       meta.Width,
		Height:      meta.Height,
		S3Key:       fmt.Sprintf("media/%s/%s", assetID, meta.Filename),
		SHA256:      sha256Hash,
		PinStatus:   string(domain.PinPending),
		PinAttempts: 0,
		RefCount:    1,
		CreatedAt:   time.Now(),
	}

	// Try to find existing asset by SHA256 (deduplication)
	existingAsset, isDedup, err := s.repository.FindOrCreateBySHA256(ctx, asset)
	if err != nil {
		return nil, false, fmt.Errorf("failed to check/create asset: %w", err)
	}

	if isDedup {
		return existingAsset, true, nil
	}

	contentReader := io.NopCloser(bytes.NewReader(content))
	pinResult, err := s.pinner.PinFile(ctx, contentReader, meta.Filename)
	if err != nil {
		// Update asset with pin failure
		errorMsg := err.Error()
		asset.PinStatus = string(domain.PinFailed)
		asset.PinError = &errorMsg
		asset.PinAttempts = 1

		// Update the asset in repository
		if updateErr := s.repository.UpdateAfterUpload(ctx, asset.ID, asset.Mime, asset.Bytes, asset.Width, asset.Height); updateErr != nil {
			return nil, false, fmt.Errorf("failed to update asset after pin failure: %w", updateErr)
		}

		return nil, false, fmt.Errorf("failed to pin file: %w", err)
	}

	// Update asset with pin success
	asset.IPFSCID = &pinResult.CID
	asset.PinStatus = string(domain.PinPinned)
	asset.PinAttempts = 1

	// Set gateway URL if available
	if gatewayURL := s.pinner.GatewayURL(pinResult.CID); gatewayURL != "" {
		asset.GatewayURL = &gatewayURL
	}

	// Update the asset in repository with final pin result
	if err := s.repository.SetPinned(ctx, asset.ID, pinResult.CID, asset.GatewayURL); err != nil {
		return nil, false, fmt.Errorf("failed to update asset with pin result: %w", err)
	}

	return asset, false, nil
}

func (s *Service) GetAsset(ctx context.Context, id string) (*domain.AssetDoc, error) {
	return s.repository.GetByID(ctx, id)
}

func (s *Service) GetAssetByCID(ctx context.Context, cid string) (*domain.AssetDoc, error) {
	return s.repository.GetByCID(ctx, cid)
}

func (s *Service) UploadMedia(ctx context.Context, files []io.Reader, metas []domain.UploadMeta) ([]*domain.AssetDoc, error) {
	if len(files) != len(metas) {
		return nil, fmt.Errorf("files and metadata count mismatch")
	}

	results := make([]*domain.AssetDoc, 0, len(files))

	for i, file := range files {
		meta := metas[i]
		
		// Sanitize filename
		meta.Filename = utils.SanitizeFilename(meta.Filename)
		
		// Validate file type
		if err := utils.ValidateFileType(meta.Filename, meta.Mime); err != nil {
			return nil, fmt.Errorf("invalid file type for file %d: %w", i, err)
		}
		
		// Get media type for validation
		mediaType := utils.GetMediaType(meta.Mime)
		
		// Read content for processing
		content, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %d: %w", i, err)
		}
		
		// Validate file size
		if err := utils.ValidateFileSize(int64(len(content)), mediaType); err != nil {
			return nil, fmt.Errorf("file %d size validation failed: %w", i, err)
		}
		
		// Validate image dimensions if applicable
		if utils.IsImageMimeType(meta.Mime) && meta.Width != nil && meta.Height != nil {
			if err := utils.ValidateImageDimensions(*meta.Width, *meta.Height); err != nil {
				return nil, fmt.Errorf("file %d dimension validation failed: %w", i, err)
			}
		}

		// Calculate SHA256 for deduplication
		hash := sha256.Sum256(content)
		sha256Hash := hex.EncodeToString(hash[:])

		// Generate asset ID and S3 key
		assetID := uuid.New().String()
		s3Key := fmt.Sprintf("media/%s/%s", assetID, meta.Filename)

		// Create asset document
		asset := &domain.AssetDoc{
			ID:          assetID,
			Kind:        meta.Kind,
			Mime:        meta.Mime,
			Bytes:       int64(len(content)),
			Width:       meta.Width,
			Height:      meta.Height,
			S3Key:       s3Key,
			SHA256:      sha256Hash,
			PinStatus:   string(domain.PinPending),
			PinAttempts: 0,
			RefCount:    1,
			CreatedAt:   time.Now(),
		}

		// Check for existing asset by SHA256 (deduplication)
		existingAsset, isDedup, err := s.repository.FindOrCreateBySHA256(ctx, asset)
		if err != nil {
			return nil, fmt.Errorf("failed to check/create asset %d: %w", i, err)
		}

		if isDedup {
			results = append(results, existingAsset)
			continue
		}

		// Upload to S3
		storageResult, err := s.storage.Upload(
			ctx,
			s3Key,
			bytes.NewReader(content),
			meta.Mime,
			map[string]string{
				"asset_id": assetID,
				"kind":     meta.Kind,
				"sha256":   sha256Hash,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("failed to upload to S3: %w", err)
		}

		// Update asset with S3 URLs
		if storageResult.CDNURL != "" {
			cdnURL := storageResult.CDNURL
			asset.Variants = []domain.AssetVariantDoc{
				{
					ID:     "original",
					CDNURL: cdnURL,
					Width:  uint32(meta.Width != nil ? *meta.Width : 0),
					Height: uint32(meta.Height != nil ? *meta.Height : 0),
					Format: getFormatFromMime(meta.Mime),
				},
			}
		}

		// Pin to IPFS asynchronously
		go func(asset *domain.AssetDoc, content []byte, filename string) {
			ctx := context.Background() // Use new context for async operation
			
			contentReader := bytes.NewReader(content)
			pinResult, err := s.pinner.PinFile(ctx, contentReader, filename)
			if err != nil {
				// Update asset with pin failure
				errorMsg := err.Error()
				asset.PinStatus = string(domain.PinFailed)
				asset.PinError = &errorMsg
				s.repository.UpdateAfterUpload(ctx, asset.ID, asset.Mime, asset.Bytes, asset.Width, asset.Height)
				return
			}

			// Update asset with pin success
			if gatewayURL := s.pinner.GatewayURL(pinResult.CID); gatewayURL != "" {
				s.repository.SetPinned(ctx, asset.ID, pinResult.CID, &gatewayURL)
			} else {
				s.repository.SetPinned(ctx, asset.ID, pinResult.CID, nil)
			}
		}(asset, content, meta.Filename)

		results = append(results, asset)
	}

	return results, nil
}

func getFormatFromMime(mime string) string {
	switch mime {
	case "image/jpeg":
		return "JPG"
	case "image/png":
		return "PNG"
	case "image/gif":
		return "GIF"
	case "image/webp":
		return "WEBP"
	case "video/mp4":
		return "MP4"
	default:
		// Extract format from mime type
		parts := strings.Split(mime, "/")
		if len(parts) > 1 {
			return strings.ToUpper(parts[1])
		}
		return "UNKNOWN"
	}
}
