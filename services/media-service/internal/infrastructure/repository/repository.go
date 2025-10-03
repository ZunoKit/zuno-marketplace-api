package repository

import (
	"context"

	"github.com/quangdang46/NFT-Marketplace/services/media-service/internal/domain"
	sharedMongo "github.com/quangdang46/NFT-Marketplace/shared/mongo"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	assetsCollection = "media.assets"
)

type Repository struct {
	client *sharedMongo.MongoDB
}

func NewMediaRepository(db *sharedMongo.MongoDB) domain.MediaRepository {
	return &Repository{
		client: db,
	}
}

func (r *Repository) coll() *mongo.Collection {
	return r.client.GetDatabase().Collection(assetsCollection)
}

// Create new asset after upload; return duplicate error if unique(SHA256) violated.
func (r *Repository) Create(ctx context.Context, a *domain.AssetDoc) error {
	_, err := r.coll().InsertOne(ctx, a)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return domain.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// Fetch
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.AssetDoc, error) {
	var out domain.AssetDoc
	err := r.coll().FindOne(ctx, bson.M{"_id": id}).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, domain.ErrAssetNotFound
	}
	return &out, err
}

func (r *Repository) GetByCID(ctx context.Context, cid string) (*domain.AssetDoc, error) {
	var out domain.AssetDoc
	err := r.coll().FindOne(ctx, bson.M{"ipfs_cid": cid}).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, domain.ErrAssetNotFound
	}
	return &out, err
}

// Idempotent create by SHA256 (dedup)
func (r *Repository) FindOrCreateBySHA256(ctx context.Context, a *domain.AssetDoc) (asset *domain.AssetDoc, dedup bool, err error) {
	var existing domain.AssetDoc
	err = r.coll().FindOne(ctx, bson.M{"sha256": a.SHA256}).Decode(&existing)
	if err == nil {
		return &existing, true, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, false, err
	}
	_, err = r.coll().InsertOne(ctx, a)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			if e := r.coll().FindOne(ctx, bson.M{"sha256": a.SHA256}).Decode(&existing); e == nil {
				return &existing, true, nil
			}
		}
		return nil, false, err
	}
	return a, false, nil
}

// Update detected props after upload
func (r *Repository) UpdateAfterUpload(ctx context.Context, id, mime string, bytes int64, w, h *uint32) error {
	set := bson.M{"mime": mime, "bytes": bytes}
	if w != nil {
		set["width"] = *w
	} else {
		set["width"] = nil
	}
	if h != nil {
		set["height"] = *h
	} else {
		set["height"] = nil
	}
	res, err := r.coll().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrAssetNotFound
	}
	return nil
}

// Set final pin result (Pinata SYNC path)
func (r *Repository) SetPinned(ctx context.Context, id, cid string, gatewayURL *string) error {
	set := bson.M{"pin_status": "PINNED", "ipfs_cid": cid}
	if gatewayURL != nil {
		set["gateway_url"] = *gatewayURL
	}
	res, err := r.coll().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": set})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return domain.ErrAssetNotFound
	}
	return nil
}

// Paging (admin/debug)
func (r *Repository) List(ctx context.Context, filter map[string]any, pageSize int, pageToken string) (items []domain.AssetDoc, next string, err error) {
	findFilter := bson.M(filter)
	opts := options.Find().SetLimit(int64(pageSize))
	cur, err := r.coll().Find(ctx, findFilter, opts)
	if err != nil {
		return nil, "", err
	}
	defer cur.Close(ctx)
	for cur.Next(ctx) {
		var a domain.AssetDoc
		if err := cur.Decode(&a); err != nil {
			return nil, "", err
		}
		items = append(items, a)
	}
	return items, "", cur.Err()
}
