package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/service"
)

// Mock implementations for testing
type mockMediaRepository struct {
	assets map[string]*domain.AssetDoc
	sha256 map[string]*domain.AssetDoc
}

func newMockMediaRepository() *mockMediaRepository {
	return &mockMediaRepository{
		assets: make(map[string]*domain.AssetDoc),
		sha256: make(map[string]*domain.AssetDoc),
	}
}

func (m *mockMediaRepository) Create(ctx context.Context, a *domain.AssetDoc) error {
	if _, exists := m.assets[a.ID]; exists {
		return domain.ErrAlreadyExists
	}
	m.assets[a.ID] = a
	m.sha256[a.SHA256] = a
	return nil
}

func (m *mockMediaRepository) GetByID(ctx context.Context, id string) (*domain.AssetDoc, error) {
	if asset, exists := m.assets[id]; exists {
		return asset, nil
	}
	return nil, domain.ErrAssetNotFound
}

func (m *mockMediaRepository) GetByCID(ctx context.Context, cid string) (*domain.AssetDoc, error) {
	for _, asset := range m.assets {
		if asset.IPFSCID != nil && *asset.IPFSCID == cid {
			return asset, nil
		}
	}
	return nil, domain.ErrAssetNotFound
}

func (m *mockMediaRepository) FindOrCreateBySHA256(ctx context.Context, a *domain.AssetDoc) (*domain.AssetDoc, bool, error) {
	if existing, exists := m.sha256[a.SHA256]; exists {
		return existing, true, nil
	}
	m.assets[a.ID] = a
	m.sha256[a.SHA256] = a
	return a, false, nil
}

func (m *mockMediaRepository) UpdateAfterUpload(ctx context.Context, id, mime string, bytes int64, w, h *uint32) error {
	if asset, exists := m.assets[id]; exists {
		asset.Mime = mime
		asset.Bytes = bytes
		asset.Width = w
		asset.Height = h
		return nil
	}
	return domain.ErrAssetNotFound
}

func (m *mockMediaRepository) SetPinned(ctx context.Context, id, cid string, gatewayURL *string) error {
	if asset, exists := m.assets[id]; exists {
		asset.IPFSCID = &cid
		asset.PinStatus = string(domain.PinPinned)
		asset.GatewayURL = gatewayURL
		return nil
	}
	return domain.ErrAssetNotFound
}

func (m *mockMediaRepository) List(ctx context.Context, filter map[string]any, pageSize int, pageToken string) ([]domain.AssetDoc, string, error) {
	var assets []domain.AssetDoc
	for _, asset := range m.assets {
		assets = append(assets, *asset)
	}
	return assets, "", nil
}

type mockPinner struct {
	shouldFail bool
}

func newMockPinner(shouldFail bool) *mockPinner {
	return &mockPinner{shouldFail: shouldFail}
}

func (m *mockPinner) PinFile(ctx context.Context, r io.Reader, name string) (domain.PinResult, error) {
	if m.shouldFail {
		return domain.PinResult{}, errors.New("pin failed")
	}

	// Read content to simulate pinning
	content, _ := io.ReadAll(r)

	return domain.PinResult{
		CID:         "QmTestCID123456789",
		Size:        int64(len(content)),
		IsDuplicate: false,
	}, nil
}

func (m *mockPinner) PinJSON(ctx context.Context, v any, name string) (domain.PinResult, error) {
	if m.shouldFail {
		return domain.PinResult{}, errors.New("pin failed")
	}
	return domain.PinResult{
		CID:         "QmTestJSONCID123456789",
		Size:        100,
		IsDuplicate: false,
	}, nil
}

func (m *mockPinner) Unpin(ctx context.Context, cid string) error {
	if m.shouldFail {
		return errors.New("unpin failed")
	}
	return nil
}

func (m *mockPinner) GatewayURL(cid string) string {
	return "https://gateway.pinata.cloud/ipfs/" + cid
}

// Test cases
func TestUploadAndPin_Success(t *testing.T) {
	repo := newMockMediaRepository()
	pinner := newMockPinner(false)
	svc := service.NewMediaService(repo, pinner)

	ctx := context.Background()
	meta := domain.UploadMeta{
		Filename: "test.jpg",
		Mime:     "image/jpeg",
		Kind:     "IMAGE",
		Width:    uint32Ptr(1920),
		Height:   uint32Ptr(1080),
	}
	content := []byte("test image content")

	asset, dedup, err := svc.UploadAndPin(ctx, meta, bytes.NewReader(content), int64(len(content)))

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if dedup {
		t.Fatal("Expected not deduplicated")
	}

	if asset == nil {
		t.Fatal("Expected asset to be returned")
	}

	if asset.Mime != meta.Mime {
		t.Errorf("Expected mime %s, got %s", meta.Mime, asset.Mime)
	}

	if asset.PinStatus != string(domain.PinPinned) {
		t.Errorf("Expected pin status %s, got %s", domain.PinPinned, asset.PinStatus)
	}

	if asset.IPFSCID == nil {
		t.Fatal("Expected IPFS CID to be set")
	}
}

func TestUploadAndPin_Deduplication(t *testing.T) {
	repo := newMockMediaRepository()
	pinner := newMockPinner(false)
	svc := service.NewMediaService(repo, pinner)

	ctx := context.Background()
	meta := domain.UploadMeta{
		Filename: "test.jpg",
		Mime:     "image/jpeg",
		Kind:     "IMAGE",
	}
	content := []byte("test image content")

	// First upload
	asset1, _, err1 := svc.UploadAndPin(ctx, meta, bytes.NewReader(content), int64(len(content)))
	if err1 != nil {
		t.Fatalf("First upload failed: %v", err1)
	}

	// Second upload with same content
	asset2, dedup2, err2 := svc.UploadAndPin(ctx, meta, bytes.NewReader(content), int64(len(content)))
	if err2 != nil {
		t.Fatalf("Second upload failed: %v", err2)
	}

	if !dedup2 {
		t.Fatal("Expected second upload to be deduplicated")
	}

	if asset1.ID != asset2.ID {
		t.Errorf("Expected same asset ID, got %s and %s", asset1.ID, asset2.ID)
	}
}

func TestUploadAndPin_Validation(t *testing.T) {
	repo := newMockMediaRepository()
	pinner := newMockPinner(false)
	svc := service.NewMediaService(repo, pinner)

	ctx := context.Background()
	meta := domain.UploadMeta{
		Filename: "test.jpg",
		Mime:     "image/jpeg",
		Kind:     "IMAGE",
	}

	// Test nil reader
	_, _, err := svc.UploadAndPin(ctx, meta, nil, 0)
	if err == nil {
		t.Fatal("Expected error for nil reader")
	}

	// Test empty filename
	meta.Filename = ""
	_, _, err = svc.UploadAndPin(ctx, meta, bytes.NewReader([]byte("test")), 4)
	if err == nil {
		t.Fatal("Expected error for empty filename")
	}

	// Test empty mime
	meta.Filename = "test.jpg"
	meta.Mime = ""
	_, _, err = svc.UploadAndPin(ctx, meta, bytes.NewReader([]byte("test")), 4)
	if err == nil {
		t.Fatal("Expected error for empty mime")
	}
}

func TestUploadAndPin_PinFailure(t *testing.T) {
	repo := newMockMediaRepository()
	pinner := newMockPinner(true) // Will fail
	svc := service.NewMediaService(repo, pinner)

	ctx := context.Background()
	meta := domain.UploadMeta{
		Filename: "test.jpg",
		Mime:     "image/jpeg",
		Kind:     "IMAGE",
	}
	content := []byte("test image content")

	_, _, err := svc.UploadAndPin(ctx, meta, bytes.NewReader(content), int64(len(content)))

	if err == nil {
		t.Fatal("Expected error for pin failure")
	}
}

func TestGetAsset(t *testing.T) {
	repo := newMockMediaRepository()
	pinner := newMockPinner(false)
	svc := service.NewMediaService(repo, pinner)

	ctx := context.Background()

	// Create a test asset
	testAsset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		CreatedAt: time.Now(),
	}
	repo.assets[testAsset.ID] = testAsset

	// Test successful retrieval
	asset, err := svc.GetAsset(ctx, "test-id")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if asset.ID != testAsset.ID {
		t.Errorf("Expected asset ID %s, got %s", testAsset.ID, asset.ID)
	}

	// Test not found
	_, err = svc.GetAsset(ctx, "non-existent")
	if err != domain.ErrAssetNotFound {
		t.Errorf("Expected ErrAssetNotFound, got %v", err)
	}
}

func TestGetAssetByCID(t *testing.T) {
	repo := newMockMediaRepository()
	pinner := newMockPinner(false)
	svc := service.NewMediaService(repo, pinner)

	ctx := context.Background()

	// Create a test asset with CID
	cid := "QmTestCID123456789"
	testAsset := &domain.AssetDoc{
		ID:        "test-id",
		Kind:      "IMAGE",
		Mime:      "image/jpeg",
		Bytes:     1024,
		IPFSCID:   &cid,
		CreatedAt: time.Now(),
	}
	repo.assets[testAsset.ID] = testAsset

	// Test successful retrieval
	asset, err := svc.GetAssetByCID(ctx, cid)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if asset.ID != testAsset.ID {
		t.Errorf("Expected asset ID %s, got %s", testAsset.ID, asset.ID)
	}

	// Test not found
	_, err = svc.GetAssetByCID(ctx, "non-existent-cid")
	if err != domain.ErrAssetNotFound {
		t.Errorf("Expected ErrAssetNotFound, got %v", err)
	}
}

// Helper function
func uint32Ptr(v uint32) *uint32 {
	return &v
}
