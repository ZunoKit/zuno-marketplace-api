package test

import (
	"context"
	"testing"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
)

func TestMockRepository_Create(t *testing.T) {
	repo := newMockMediaRepository()
	ctx := context.Background()

	asset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		SHA256:    "test-sha256",
		CreatedAt: time.Now(),
	}

	// Test successful creation
	err := repo.Create(ctx, asset)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test duplicate creation
	err = repo.Create(ctx, asset)
	if err != domain.ErrAlreadyExists {
		t.Errorf("Expected ErrAlreadyExists, got %v", err)
	}
}

func TestMockRepository_GetByID(t *testing.T) {
	repo := newMockMediaRepository()
	ctx := context.Background()

	asset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		CreatedAt: time.Now(),
	}
	repo.assets[asset.ID] = asset

	// Test successful retrieval
	retrieved, err := repo.GetByID(ctx, "test-id")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrieved.ID != asset.ID {
		t.Errorf("Expected asset ID %s, got %s", asset.ID, retrieved.ID)
	}

	// Test not found
	_, err = repo.GetByID(ctx, "non-existent")
	if err != domain.ErrAssetNotFound {
		t.Errorf("Expected ErrAssetNotFound, got %v", err)
	}
}

func TestMockRepository_GetByCID(t *testing.T) {
	repo := newMockMediaRepository()
	ctx := context.Background()

	cid := "QmTestCID123456789"
	asset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		IPFSCID:   &cid,
		CreatedAt: time.Now(),
	}
	repo.assets[asset.ID] = asset

	// Test successful retrieval
	retrieved, err := repo.GetByCID(ctx, cid)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if retrieved.ID != asset.ID {
		t.Errorf("Expected asset ID %s, got %s", asset.ID, retrieved.ID)
	}

	// Test not found
	_, err = repo.GetByCID(ctx, "non-existent-cid")
	if err != domain.ErrAssetNotFound {
		t.Errorf("Expected ErrAssetNotFound, got %v", err)
	}
}

func TestMockRepository_FindOrCreateBySHA256(t *testing.T) {
	repo := newMockMediaRepository()
	ctx := context.Background()

	sha256 := "test-sha256"
	asset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		SHA256:    sha256,
		CreatedAt: time.Now(),
	}

	// Test creation of new asset
	retrieved, dedup, err := repo.FindOrCreateBySHA256(ctx, asset)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if dedup {
		t.Fatal("Expected not deduplicated")
	}

	if retrieved.ID != asset.ID {
		t.Errorf("Expected asset ID %s, got %s", asset.ID, retrieved.ID)
	}

	// Test deduplication
	asset2 := &domain.AssetDoc{
		ID:        "test-id-2",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		SHA256:    sha256, // Same SHA256
		CreatedAt: time.Now(),
	}

	retrieved2, dedup2, err := repo.FindOrCreateBySHA256(ctx, asset2)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !dedup2 {
		t.Fatal("Expected deduplicated")
	}

	if retrieved2.ID != asset.ID {
		t.Errorf("Expected original asset ID %s, got %s", asset.ID, retrieved2.ID)
	}
}

func TestMockRepository_UpdateAfterUpload(t *testing.T) {
	repo := newMockMediaRepository()
	ctx := context.Background()

	asset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		CreatedAt: time.Now(),
	}
	repo.assets[asset.ID] = asset

	width := uint32(1920)
	height := uint32(1080)

	// Test successful update
	err := repo.UpdateAfterUpload(ctx, "test-id", "image/png", 2048, &width, &height)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	updated := repo.assets["test-id"]
	if updated.Mime != "image/png" {
		t.Errorf("Expected mime image/png, got %s", updated.Mime)
	}

	if updated.Bytes != 2048 {
		t.Errorf("Expected bytes 2048, got %d", updated.Bytes)
	}

	if *updated.Width != width {
		t.Errorf("Expected width %d, got %d", width, *updated.Width)
	}

	if *updated.Height != height {
		t.Errorf("Expected height %d, got %d", height, *updated.Height)
	}

	// Test not found
	err = repo.UpdateAfterUpload(ctx, "non-existent", "image/png", 2048, &width, &height)
	if err != domain.ErrAssetNotFound {
		t.Errorf("Expected ErrAssetNotFound, got %v", err)
	}
}

func TestMockRepository_SetPinned(t *testing.T) {
	repo := newMockMediaRepository()
	ctx := context.Background()

	asset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		CreatedAt: time.Now(),
	}
	repo.assets[asset.ID] = asset

	cid := "QmTestCID123456789"
	gatewayURL := "https://gateway.pinata.cloud/ipfs/QmTestCID123456789"

	// Test successful pin
	err := repo.SetPinned(ctx, "test-id", cid, &gatewayURL)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	updated := repo.assets["test-id"]
	if *updated.IPFSCID != cid {
		t.Errorf("Expected CID %s, got %s", cid, *updated.IPFSCID)
	}

	if updated.PinStatus != string(domain.PinPinned) {
		t.Errorf("Expected pin status %s, got %s", domain.PinPinned, updated.PinStatus)
	}

	if *updated.GatewayURL != gatewayURL {
		t.Errorf("Expected gateway URL %s, got %s", gatewayURL, *updated.GatewayURL)
	}

	// Test not found
	err = repo.SetPinned(ctx, "non-existent", cid, &gatewayURL)
	if err != domain.ErrAssetNotFound {
		t.Errorf("Expected ErrAssetNotFound, got %v", err)
	}
}

func TestMockRepository_List(t *testing.T) {
	repo := newMockMediaRepository()
	ctx := context.Background()

	// Add some test assets
	asset1 := &domain.AssetDoc{
		ID:        "test-id-1",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		CreatedAt: time.Now(),
	}
	asset2 := &domain.AssetDoc{
		ID:        "test-id-2",
		Kind:      "VIDEO",
		Mime:      "video/mp4",
		Bytes:     2048,
		CreatedAt: time.Now(),
	}

	repo.assets[asset1.ID] = asset1
	repo.assets[asset2.ID] = asset2

	// Test listing all assets
	assets, next, err := repo.List(ctx, map[string]any{}, 10, "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(assets) != 2 {
		t.Errorf("Expected 2 assets, got %d", len(assets))
	}

	if next != "" {
		t.Errorf("Expected empty next token, got %s", next)
	}

	// Test filtering - Note: mock implementation doesn't support filtering yet
	// This test verifies the basic List functionality works
	filter := map[string]any{"kind": "IMAGE"}
	assets, next, err = repo.List(ctx, filter, 10, "")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Mock implementation returns all assets regardless of filter
	// In real implementation, this would be filtered
	if len(assets) != 2 {
		t.Errorf("Expected 2 assets (mock doesn't filter), got %d", len(assets))
	}
}
