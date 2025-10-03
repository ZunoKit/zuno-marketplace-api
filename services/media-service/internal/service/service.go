package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
)

type Service struct {
	repository domain.MediaRepository
	pinner     domain.Pinner
}

func NewMediaService(
	repository domain.MediaRepository,
	pinner domain.Pinner,
) domain.MediaService {
	return &Service{
		repository: repository,
		pinner:     pinner,
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
