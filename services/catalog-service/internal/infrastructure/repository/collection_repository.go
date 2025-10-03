package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/quangdang46/NFT-Marketplace/services/catalog-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type CollectionRepository struct {
	postgresDb *postgres.Postgres
	redisDb    *redis.Redis
}

// NewCollectionRepository creates a new PostgreSQL collection repository
func NewCollectionRepository(postgresDb *postgres.Postgres, redisDb *redis.Redis) domain.CollectionsRepository {
	return &CollectionRepository{
		postgresDb: postgresDb,
		redisDb:    redisDb,
	}
}

func (r *CollectionRepository) Upsert(ctx context.Context, c domain.Collection) (created bool, err error) {
	// Initialize big.Int fields if they are nil
	if c.MaxSupply == nil {
		c.MaxSupply = big.NewInt(0)
	}
	if c.TotalSupply == nil {
		c.TotalSupply = big.NewInt(0)
	}
	if c.FloorPrice == nil {
		c.FloorPrice = big.NewInt(0)
	}
	if c.VolumeTraded == nil {
		c.VolumeTraded = big.NewInt(0)
	}
	// Initialize minting fields
	if c.MintPrice == nil {
		c.MintPrice = big.NewInt(0)
	}
	if c.RoyaltyFee == nil {
		c.RoyaltyFee = big.NewInt(0)
	}
	if c.MintLimitPerWallet == nil {
		c.MintLimitPerWallet = big.NewInt(0)
	}
	if c.MintStartTime == nil {
		c.MintStartTime = big.NewInt(0)
	}
	if c.AllowlistMintPrice == nil {
		c.AllowlistMintPrice = big.NewInt(0)
	}
	if c.PublicMintPrice == nil {
		c.PublicMintPrice = big.NewInt(0)
	}
	if c.AllowlistStageDuration == nil {
		c.AllowlistStageDuration = big.NewInt(0)
	}

	// Check if collection exists first
	existing, err := r.GetByPK(ctx, domain.ChainID(c.ChainID), domain.Address(c.ContractAddress))
	if err != nil && err != sql.ErrNoRows {
		return false, fmt.Errorf("failed to check existing collection: %w", err)
	}

	now := time.Now()
	if existing.ID == "" {
		// Create new collection
		c.ID = uuid.New().String()
		c.CreatedAt = now
		c.UpdatedAt = now

		query := `
			INSERT INTO collections (
				id, slug, name, description, chain_id, contract_address, creator, tx_hash, owner,
				collection_type, max_supply, total_supply, royalty_recipient, royalty_percentage,
				mint_price, royalty_fee, mint_limit_per_wallet, mint_start_time,
				allowlist_mint_price, public_mint_price, allowlist_stage_duration, token_uri,
				is_verified, is_explicit, is_featured, image_url, banner_url, external_url,
				discord_url, twitter_url, instagram_url, telegram_url, floor_price, volume_traded,
				created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36
			)
		`

		_, err = r.postgresDb.GetClient().ExecContext(ctx, query,
			c.ID, c.Slug, c.Name, c.Description, c.ChainID, c.ContractAddress, c.Creator, c.TxHash, c.Owner,
			c.CollectionType, c.MaxSupply.String(), c.TotalSupply.String(), c.RoyaltyRecipient, c.RoyaltyPercentage,
			c.MintPrice.String(), c.RoyaltyFee.String(), c.MintLimitPerWallet.String(), c.MintStartTime.String(),
			c.AllowlistMintPrice.String(), c.PublicMintPrice.String(), c.AllowlistStageDuration.String(), c.TokenURI,
			c.IsVerified, c.IsExplicit, c.IsFeatured, c.ImageURL, c.BannerURL, c.ExternalURL,
			c.DiscordURL, c.TwitterURL, c.InstagramURL, c.TelegramURL, c.FloorPrice.String(), c.VolumeTraded.String(),
			c.CreatedAt, c.UpdatedAt,
		)
		if err != nil {
			return false, fmt.Errorf("failed to insert collection: %w", err)
		}

		// Also insert into collection_bindings
		bindingQuery := `
			INSERT INTO collection_bindings (
				id, collection_id, chain_id, family, token_standard, contract_address, is_primary
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`
		_, err = r.postgresDb.GetClient().ExecContext(ctx, bindingQuery,
			uuid.New().String(), c.ID, c.ChainID, "evm", c.CollectionType, c.ContractAddress, true,
		)
		if err != nil {
			return false, fmt.Errorf("failed to insert collection binding: %w", err)
		}

		created = true
	} else {
		// Update existing collection
		c.UpdatedAt = now
		c.ID = existing.ID // Preserve existing ID

		query := `
			UPDATE collections SET
				slug = $1, name = $2, description = $3, creator = $4, tx_hash = $5, owner = $6,
				collection_type = $7, max_supply = $8, total_supply = $9, royalty_recipient = $10, royalty_percentage = $11,
				mint_price = $12, royalty_fee = $13, mint_limit_per_wallet = $14, mint_start_time = $15,
				allowlist_mint_price = $16, public_mint_price = $17, allowlist_stage_duration = $18, token_uri = $19,
				is_verified = $20, is_explicit = $21, is_featured = $22, image_url = $23, banner_url = $24, external_url = $25,
				discord_url = $26, twitter_url = $27, instagram_url = $28, telegram_url = $29, floor_price = $30, volume_traded = $31,
				updated_at = $32
			WHERE chain_id = $33 AND contract_address = $34
		`

		_, err = r.postgresDb.GetClient().ExecContext(ctx, query,
			c.Slug, c.Name, c.Description, c.Creator, c.TxHash, c.Owner,
			c.CollectionType, c.MaxSupply.String(), c.TotalSupply.String(), c.RoyaltyRecipient, c.RoyaltyPercentage,
			c.MintPrice.String(), c.RoyaltyFee.String(), c.MintLimitPerWallet.String(), c.MintStartTime.String(),
			c.AllowlistMintPrice.String(), c.PublicMintPrice.String(), c.AllowlistStageDuration.String(), c.TokenURI,
			c.IsVerified, c.IsExplicit, c.IsFeatured, c.ImageURL, c.BannerURL, c.ExternalURL,
			c.DiscordURL, c.TwitterURL, c.InstagramURL, c.TelegramURL, c.FloorPrice.String(), c.VolumeTraded.String(),
			c.UpdatedAt, c.ChainID, c.ContractAddress,
		)
		if err != nil {
			return false, fmt.Errorf("failed to update collection: %w", err)
		}
	}

	// Invalidate cache
	cacheKey := fmt.Sprintf("collection:%s:%s", c.ChainID, c.ContractAddress)
	r.redisDb.Delete(ctx, cacheKey)

	return created, nil
}

func (r *CollectionRepository) GetByPK(ctx context.Context, chainID domain.ChainID, contract domain.Address) (domain.Collection, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("collection:%s:%s", chainID, contract)
	cached, err := r.redisDb.Get(ctx, cacheKey)
	if err == nil && cached != "" {
		// TODO: Implement JSON unmarshaling for cached data
		// For now, we'll just query the database
	}

	query := `
		SELECT 
			c.id, c.slug, c.name, c.description, c.chain_id, c.contract_address, c.creator, c.tx_hash, c.owner,
			c.collection_type, c.max_supply, c.total_supply, c.royalty_recipient, c.royalty_percentage,
			c.mint_price, c.royalty_fee, c.mint_limit_per_wallet, c.mint_start_time,
			c.allowlist_mint_price, c.public_mint_price, c.allowlist_stage_duration, c.token_uri,
			c.is_verified, c.is_explicit, c.is_featured, c.image_url, c.banner_url, c.external_url,
			c.discord_url, c.twitter_url, c.instagram_url, c.telegram_url, c.floor_price, c.volume_traded,
			c.created_at, c.updated_at
		FROM collections c
		WHERE c.chain_id = $1 AND c.contract_address = $2
	`

	var collection domain.Collection
	var maxSupplyStr, totalSupplyStr, floorPriceStr, volumeTradedStr sql.NullString
	var mintPriceStr, royaltyFeeStr, mintLimitPerWalletStr, mintStartTimeStr sql.NullString
	var allowlistMintPriceStr, publicMintPriceStr, allowlistStageDurationStr sql.NullString

	err = r.postgresDb.GetClient().QueryRowContext(ctx, query, string(chainID), string(contract)).Scan(
		&collection.ID, &collection.Slug, &collection.Name, &collection.Description, &collection.ChainID, &collection.ContractAddress, &collection.Creator, &collection.TxHash, &collection.Owner,
		&collection.CollectionType, &maxSupplyStr, &totalSupplyStr, &collection.RoyaltyRecipient, &collection.RoyaltyPercentage,
		&mintPriceStr, &royaltyFeeStr, &mintLimitPerWalletStr, &mintStartTimeStr,
		&allowlistMintPriceStr, &publicMintPriceStr, &allowlistStageDurationStr, &collection.TokenURI,
		&collection.IsVerified, &collection.IsExplicit, &collection.IsFeatured, &collection.ImageURL, &collection.BannerURL, &collection.ExternalURL,
		&collection.DiscordURL, &collection.TwitterURL, &collection.InstagramURL, &collection.TelegramURL, &floorPriceStr, &volumeTradedStr,
		&collection.CreatedAt, &collection.UpdatedAt,
	)

	if err != nil {
		return domain.Collection{}, err
	}

	// Parse big.Int fields with proper initialization
	collection.MaxSupply = new(big.Int)
	if maxSupplyStr.Valid && maxSupplyStr.String != "" {
		if _, ok := collection.MaxSupply.SetString(maxSupplyStr.String, 10); !ok {
			collection.MaxSupply.SetInt64(0)
		}
	} else {
		collection.MaxSupply.SetInt64(0)
	}

	collection.TotalSupply = new(big.Int)
	if totalSupplyStr.Valid && totalSupplyStr.String != "" {
		if _, ok := collection.TotalSupply.SetString(totalSupplyStr.String, 10); !ok {
			collection.TotalSupply.SetInt64(0)
		}
	} else {
		collection.TotalSupply.SetInt64(0)
	}

	collection.FloorPrice = new(big.Int)
	if floorPriceStr.Valid && floorPriceStr.String != "" {
		if _, ok := collection.FloorPrice.SetString(floorPriceStr.String, 10); !ok {
			collection.FloorPrice.SetInt64(0)
		}
	} else {
		collection.FloorPrice.SetInt64(0)
	}

	collection.VolumeTraded = new(big.Int)
	if volumeTradedStr.Valid && volumeTradedStr.String != "" {
		if _, ok := collection.VolumeTraded.SetString(volumeTradedStr.String, 10); !ok {
			collection.VolumeTraded.SetInt64(0)
		}
	} else {
		collection.VolumeTraded.SetInt64(0)
	}

	// Parse minting configuration fields
	collection.MintPrice = new(big.Int)
	if mintPriceStr.Valid && mintPriceStr.String != "" {
		if _, ok := collection.MintPrice.SetString(mintPriceStr.String, 10); !ok {
			collection.MintPrice.SetInt64(0)
		}
	} else {
		collection.MintPrice.SetInt64(0)
	}

	collection.RoyaltyFee = new(big.Int)
	if royaltyFeeStr.Valid && royaltyFeeStr.String != "" {
		if _, ok := collection.RoyaltyFee.SetString(royaltyFeeStr.String, 10); !ok {
			collection.RoyaltyFee.SetInt64(0)
		}
	} else {
		collection.RoyaltyFee.SetInt64(0)
	}

	collection.MintLimitPerWallet = new(big.Int)
	if mintLimitPerWalletStr.Valid && mintLimitPerWalletStr.String != "" {
		if _, ok := collection.MintLimitPerWallet.SetString(mintLimitPerWalletStr.String, 10); !ok {
			collection.MintLimitPerWallet.SetInt64(0)
		}
	} else {
		collection.MintLimitPerWallet.SetInt64(0)
	}

	collection.MintStartTime = new(big.Int)
	if mintStartTimeStr.Valid && mintStartTimeStr.String != "" {
		if _, ok := collection.MintStartTime.SetString(mintStartTimeStr.String, 10); !ok {
			collection.MintStartTime.SetInt64(0)
		}
	} else {
		collection.MintStartTime.SetInt64(0)
	}

	collection.AllowlistMintPrice = new(big.Int)
	if allowlistMintPriceStr.Valid && allowlistMintPriceStr.String != "" {
		if _, ok := collection.AllowlistMintPrice.SetString(allowlistMintPriceStr.String, 10); !ok {
			collection.AllowlistMintPrice.SetInt64(0)
		}
	} else {
		collection.AllowlistMintPrice.SetInt64(0)
	}

	collection.PublicMintPrice = new(big.Int)
	if publicMintPriceStr.Valid && publicMintPriceStr.String != "" {
		if _, ok := collection.PublicMintPrice.SetString(publicMintPriceStr.String, 10); !ok {
			collection.PublicMintPrice.SetInt64(0)
		}
	} else {
		collection.PublicMintPrice.SetInt64(0)
	}

	collection.AllowlistStageDuration = new(big.Int)
	if allowlistStageDurationStr.Valid && allowlistStageDurationStr.String != "" {
		if _, ok := collection.AllowlistStageDuration.SetString(allowlistStageDurationStr.String, 10); !ok {
			collection.AllowlistStageDuration.SetInt64(0)
		}
	} else {
		collection.AllowlistStageDuration.SetInt64(0)
	}

	// Cache the result
	// TODO: Implement JSON marshaling for cache storage
	// r.redisDb.SetWithExpiration(ctx, cacheKey, marshaledData, time.Hour)

	return collection, nil
}
