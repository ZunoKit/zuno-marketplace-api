// media.go
package domain

import (
	"context"
	"io"
	"time"
)

//
// =============== Domain Types ===============
//

type PinStatus string

const (
	PinPending PinStatus = "PENDING"
	PinPinning PinStatus = "PINNING"
	PinPinned  PinStatus = "PINNED"
	PinFailed  PinStatus = "FAILED"
)

type UploadMeta struct {
	Filename string
	Mime     string
	Kind     string
	Width    *uint32
	Height   *uint32
	OwnerID  string // optional (for audit/link)
}

type AssetDoc struct {
	ID          string            `bson:"_id"`
	Kind        string            `bson:"kind"`
	Mime        string            `bson:"mime"`
	Bytes       int64             `bson:"bytes"`
	Width       *uint32           `bson:"width,omitempty"`
	Height      *uint32           `bson:"height,omitempty"`
	S3Key       string            `bson:"s3_key"`
	SHA256      string            `bson:"sha256"`
	IPFSCID     *string           `bson:"ipfs_cid,omitempty"`
	GatewayURL  *string           `bson:"gateway_url,omitempty"`
	PinStatus   string            `bson:"pin_status"` // expects values of PinStatus
	PinAttempts int               `bson:"pin_attempts"`
	PinError    *string           `bson:"pin_error,omitempty"`
	RefCount    uint32            `bson:"ref_count"`
	Variants    []AssetVariantDoc `bson:"variants"`
	CreatedAt   time.Time         `bson:"created_at"`
}

type AssetVariantDoc struct {
	ID     string `bson:"id"`
	CDNURL string `bson:"cdn_url"`
	Width  uint32 `bson:"width"`
	Height uint32 `bson:"height"`
	Format string `bson:"format"`
}

//
// =============== Repository (Mongo) ===============
//

type MediaRepository interface {
	// Create new asset after upload; return duplicate error if unique(SHA256) violated.
	Create(ctx context.Context, a *AssetDoc) error

	// Fetch
	GetByID(ctx context.Context, id string) (*AssetDoc, error)
	GetByCID(ctx context.Context, cid string) (*AssetDoc, error)

	// Idempotent create by SHA256 (dedup)
	FindOrCreateBySHA256(ctx context.Context, a *AssetDoc) (asset *AssetDoc, dedup bool, err error)

	// Update detected props after upload
	UpdateAfterUpload(ctx context.Context, id, mime string, bytes int64, w, h *uint32) error

	// Set final pin result (Pinata SYNC path)
	SetPinned(ctx context.Context, id, cid string, gatewayURL *string) error

	// Paging (admin/debug)
	List(ctx context.Context, filter map[string]any, pageSize int, pageToken string) (items []AssetDoc, next string, err error)
}

// =============== Infra: Pinner (Pinata) ===============
type PinResult struct {
	CID         string `json:"IpfsHash"`
	Size        int64  `json:"PinSize"`
	IsDuplicate bool   `json:"IsDuplicate"`
}

type Pinner interface {
	PinFile(ctx context.Context, r io.Reader, name string) (PinResult, error)
	PinJSON(ctx context.Context, v any, name string) (PinResult, error)
	Unpin(ctx context.Context, cid string) error
	GatewayURL(cid string) string
}

//
// =============== Service ===============
//

// MediaService covers Pinata-first (SYNC) flow and basic queries.
type MediaService interface {
	// Upload bytes + pin immediately; return asset with CID.
	UploadAndPin(ctx context.Context, meta UploadMeta, r io.Reader, sizeHint int64) (asset *AssetDoc, dedup bool, err error)

	// Queries
	GetAsset(ctx context.Context, id string) (*AssetDoc, error)
	GetAssetByCID(ctx context.Context, cid string) (*AssetDoc, error)
}
