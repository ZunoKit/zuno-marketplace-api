package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/domain"
	"github.com/quangdang46/NFT-Marketplace/services/chain-registry-service/internal/utils"
	"github.com/quangdang46/NFT-Marketplace/shared/postgres"
	"github.com/quangdang46/NFT-Marketplace/shared/redis"
)

type Repository struct {
	db    *postgres.Postgres
	redis *redis.Redis
}

func NewRepository(db *postgres.Postgres, redis *redis.Redis) domain.ChainRegistryRepository {
	return &Repository{db: db, redis: redis}
}

func (r *Repository) GetContracts(ctx context.Context, chainID domain.ChainID) (*domain.ChainContracts, error) {
	getOrInitVersion := func() string {
		versionKey := fmt.Sprintf("cache:chains:%s:version", chainID)
		if v, err := r.redis.Get(ctx, versionKey); err == nil && v != "" {
			return v
		}
		v := "1.0.0"
		_ = r.redis.SetWithExpiration(ctx, versionKey, v, 60*time.Second)
		return v
	}
	version := getOrInitVersion()
	cacheKey := fmt.Sprintf("cache:chains:%s:%s", chainID, version)
	if cached, err := r.redis.Get(ctx, cacheKey); err == nil && cached != "" {
		var result domain.ChainContracts
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return &result, nil
		}
		// If unmarshaling fails, continue to database
	}

	chainNumeric, err := utils.ParseChainID(chainID)
	if err != nil {
		return nil, fmt.Errorf("invalid chain ID: %w", err)
	}

	// Get chain info
	var chainName, nativeSymbol string
	var decimals int
	err = r.db.GetClient().QueryRowContext(ctx, QueryGetChainInfo, chainID).Scan(&chainName, &nativeSymbol, &decimals)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("chain not found: %s", chainID)
		}
		return nil, fmt.Errorf("failed to get chain info: %w", err)
	}

	// Get contracts
	rows, err := r.db.GetClient().QueryContext(ctx, QueryGetContracts, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query contracts: %w", err)
	}
	defer rows.Close()

	var contracts []domain.Contract
	for rows.Next() {
		var contract domain.Contract
		var startBlock sql.NullInt32
		var verifiedAt sql.NullTime
		var standard sql.NullString
		var implAddress sql.NullString
		var abiSha256 sql.NullString

		err := rows.Scan(
			&contract.Name,
			&contract.Address,
			&startBlock,
			&verifiedAt,
			&standard,
			&implAddress,
			&abiSha256,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}

		if startBlock.Valid {
			contract.StartBlock = uint64(startBlock.Int32)
		}
		if verifiedAt.Valid {
			contract.VerifiedAt = &verifiedAt.Time
		}
		contract.Standard = utils.DbStandardToDomain(standard)
		if implAddress.Valid {
			contract.ImplAddress = &implAddress.String
		}
		if abiSha256.Valid {
			contract.AbiSHA256 = &abiSha256.String
		}

		contracts = append(contracts, contract)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating contracts: %w", err)
	}

	// Get chain params from configuration or database
	params := domain.ChainParams{
		RequiredConfirmations: 12,
		ReorgDepth:            12,
		BlockTimeMs:           12000, // 12 seconds for most chains
	}

	result := &domain.ChainContracts{
		ChainID:         chainID,
		ChainNumeric:    uint64(chainNumeric),
		NativeSymbol:    nativeSymbol,
		Contracts:       contracts,
		Params:          params,
		RegistryVersion: version,
	}

	// Cache the result
	if cachedData, err := json.Marshal(result); err == nil {
		_ = r.redis.SetWithExpiration(ctx, cacheKey, string(cachedData), 30*time.Minute)
	}

	return result, nil
}

func (r *Repository) GetGasPolicy(ctx context.Context, chainID domain.ChainID) (*domain.ChainGasPolicy, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("chain_gas_policy:%s", chainID)
	if cached, err := r.redis.Get(ctx, cacheKey); err == nil && cached != "" {
		var result domain.ChainGasPolicy
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return &result, nil
		}
		// If unmarshaling fails, continue to database
	}

	// Get gas policy from database
	var policy domain.GasPolicy
	var updatedAt time.Time
	err := r.db.GetClient().QueryRowContext(ctx, QueryGetGasPolicy, chainID).Scan(
		&policy.MaxFeeGwei,
		&policy.PriorityFeeGwei,
		&policy.Multiplier,
		&policy.LastObservedBaseFeeGwei,
		&updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default policy if not found
			policy = domain.GasPolicy{
				MaxFeeGwei:              50.0,
				PriorityFeeGwei:         2.0,
				Multiplier:              1.1,
				LastObservedBaseFeeGwei: 20.0,
				UpdatedAt:               time.Now(),
			}
		} else {
			return nil, fmt.Errorf("failed to get gas policy: %w", err)
		}
	} else {
		policy.UpdatedAt = updatedAt
	}

	result := &domain.ChainGasPolicy{
		ChainID:         chainID,
		Policy:          policy,
		RegistryVersion: "1.0.0",
	}

	// Cache the result
	if cachedData, err := json.Marshal(result); err == nil {
		r.redis.SetWithExpiration(ctx, cacheKey, string(cachedData), 15*time.Minute)
	}

	return result, nil
}

func (r *Repository) GetRpcEndpoints(ctx context.Context, chainID domain.ChainID) (*domain.ChainRpcEndpoints, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("chain_rpc_endpoints:%s", chainID)
	if cached, err := r.redis.Get(ctx, cacheKey); err == nil && cached != "" {
		var result domain.ChainRpcEndpoints
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return &result, nil
		}
		// If unmarshaling fails, continue to database
	}

	// Get RPC endpoints from database
	rows, err := r.db.GetClient().QueryContext(ctx, QueryGetRpcEndpoints, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query RPC endpoints: %w", err)
	}
	defer rows.Close()

	var endpoints []domain.RpcEndpoint
	for rows.Next() {
		var endpoint domain.RpcEndpoint
		var authType sql.NullString
		var rateLimit sql.NullInt32

		err := rows.Scan(
			&endpoint.URL,
			&endpoint.Priority,
			&endpoint.Weight,
			&authType,
			&rateLimit,
			&endpoint.Active,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan RPC endpoint: %w", err)
		}

		endpoint.AuthType = utils.DbAuthTypeToDomain(authType)
		if rateLimit.Valid {
			rateLimitInt := int32(rateLimit.Int32)
			endpoint.RateLimit = &rateLimitInt
		}

		endpoints = append(endpoints, endpoint)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating RPC endpoints: %w", err)
	}

	result := &domain.ChainRpcEndpoints{
		ChainID:         chainID,
		Endpoints:       endpoints,
		RegistryVersion: "1.0.0",
	}

	// Cache the result
	if cachedData, err := json.Marshal(result); err == nil {
		r.redis.SetWithExpiration(ctx, cacheKey, string(cachedData), 60*time.Minute)
	}

	return result, nil
}

func (r *Repository) BumpVersion(ctx context.Context, chainID domain.ChainID, reason string) (newVersion string, err error) {
	// Generate a timestamp-based version
	newVersion = fmt.Sprintf("1.0.%d", time.Now().Unix())
	// Persist new version key (short TTL per doc)
	versionKey := fmt.Sprintf("cache:chains:%s:version", chainID)
	_ = r.redis.SetWithExpiration(ctx, versionKey, newVersion, 60*time.Second)
	// Cleanup legacy, non-versioned keys best-effort
	legacyKeys := []string{
		fmt.Sprintf("chain_contracts:%s", chainID),
		fmt.Sprintf("chain_gas_policy:%s", chainID),
		fmt.Sprintf("chain_rpc_endpoints:%s", chainID),
	}
	r.redis.Delete(ctx, legacyKeys...)
	return newVersion, nil
}

func (r *Repository) GetContractMeta(ctx context.Context, chainID domain.ChainID, address domain.Address) (*domain.ContractMeta, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("contract_meta:%s:%s", chainID, address)
	if cached, err := r.redis.Get(ctx, cacheKey); err == nil && cached != "" {
		var result domain.ContractMeta
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return &result, nil
		}
		// If unmarshaling fails, continue to database
	}

	// Get contract metadata from database
	var contract domain.Contract
	var startBlock sql.NullInt32
	var verifiedAt sql.NullTime
	var standard sql.NullString
	var implAddress sql.NullString
	var abiSha256 sql.NullString

	err := r.db.GetClient().QueryRowContext(ctx, QueryGetContractMeta, chainID, address).Scan(
		&contract.Name,
		&contract.Address,
		&startBlock,
		&verifiedAt,
		&standard,
		&implAddress,
		&abiSha256,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("contract not found: %s on chain %s", address, chainID)
		}
		return nil, fmt.Errorf("failed to get contract meta: %w", err)
	}

	if startBlock.Valid {
		contract.StartBlock = uint64(startBlock.Int32)
	}
	if verifiedAt.Valid {
		contract.VerifiedAt = &verifiedAt.Time
	}
	contract.Standard = utils.DbStandardToDomain(standard)
	if implAddress.Valid {
		contract.ImplAddress = &implAddress.String
	}
	if abiSha256.Valid {
		contract.AbiSHA256 = &abiSha256.String
	}

	result := &domain.ContractMeta{
		ChainID:         chainID,
		Contract:        contract,
		RegistryVersion: "1.0.0",
	}

	// Cache the result
	if cachedData, err := json.Marshal(result); err == nil {
		r.redis.SetWithExpiration(ctx, cacheKey, string(cachedData), 60*time.Minute)
	}

	return result, nil
}

func (r *Repository) GetAbiBlob(ctx context.Context, sha domain.Sha256) (abiJSON []byte, etag string, err error) {
	// Try cache first
	cacheKey := fmt.Sprintf("abi_blob:%s", sha)
	if cached, err := r.redis.Get(ctx, cacheKey); err == nil && cached != "" {
		// For binary data, we'll store it as base64 in Redis
		// For now, we'll go directly to the database
	}

	// Get ABI blob metadata from database
	var s3Key string
	var sizeBytes int
	err = r.db.GetClient().QueryRowContext(ctx, QueryGetAbiBlobMetadata, sha).Scan(&s3Key, &sizeBytes)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", fmt.Errorf("ABI blob not found: %s", sha)
		}
		return nil, "", fmt.Errorf("failed to get ABI blob metadata: %w", err)
	}

	// Prefer ABI stored in DB
	var abiJSONRaw sql.NullString
	if err := r.db.GetClient().QueryRowContext(ctx, QueryGetAbiBlobJson, sha).Scan(&abiJSONRaw); err == nil && abiJSONRaw.Valid && abiJSONRaw.String != "" {
		abiJSON = []byte(abiJSONRaw.String)
		etag = fmt.Sprintf("db-%s-%d", sha, time.Now().Unix())
		goto cache
	}

	// Placeholder for non-local sources (S3/IPFS could be added later)
	abiJSON = []byte(fmt.Sprintf(`{"abi":[],"metadata":{"sha256":"%s","size_bytes":%d}}`, sha, sizeBytes))
	etag = fmt.Sprintf("etag-%s-%d", sha, time.Now().Unix())

cache:
	if cachedData, err := json.Marshal(map[string]interface{}{
		"abi":       string(abiJSON),
		"etag":      etag,
		"cached_at": time.Now().Unix(),
	}); err == nil {
		r.redis.SetWithExpiration(ctx, cacheKey, string(cachedData), 24*time.Hour)
	}

	return abiJSON, etag, nil
}

func (r *Repository) ResolveProxy(ctx context.Context, chainID domain.ChainID, address domain.Address) (implAddress domain.Address, abiSha256 domain.Sha256, err error) {
	cacheKey := fmt.Sprintf("proxy_resolution:%s:%s", chainID, address)
	if cached, err := r.redis.Get(ctx, cacheKey); err == nil && cached != "" {
		var result struct {
			ImplAddress string `json:"impl_address"`
			AbiSha256   string `json:"abi_sha256"`
		}
		if err := json.Unmarshal([]byte(cached), &result); err == nil {
			return domain.Address(result.ImplAddress), domain.Sha256(result.AbiSha256), nil
		}
	}

	var implAddressStr sql.NullString
	var abiSha256Str sql.NullString
	var standard sql.NullString

	err = r.db.GetClient().QueryRowContext(ctx, QueryGetProxyContract, chainID, address).Scan(&implAddressStr, &abiSha256Str, &standard)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", fmt.Errorf("proxy contract not found: %s on chain %s", address, chainID)
		}
		return "", "", fmt.Errorf("failed to get proxy contract: %w", err)
	}

	contractStandard := utils.DbStandardToDomain(standard)
	if contractStandard != domain.StdProxy && contractStandard != domain.StdDiamond {
		return "", "", fmt.Errorf("contract %s on chain %s is not a proxy contract (standard: %s)", address, chainID, contractStandard)
	}

	if !implAddressStr.Valid || implAddressStr.String == "" {
		return "", "", fmt.Errorf("proxy contract %s on chain %s has no implementation address", address, chainID)
	}

	implAddress = domain.Address(implAddressStr.String)
	abiSha256 = domain.Sha256("")
	if abiSha256Str.Valid {
		abiSha256 = domain.Sha256(abiSha256Str.String)
	}

	if cachedData, err := json.Marshal(map[string]string{
		"impl_address": string(implAddress),
		"abi_sha256":   string(abiSha256),
	}); err == nil {
		r.redis.SetWithExpiration(ctx, cacheKey, string(cachedData), 60*time.Minute)
	}

	return implAddress, abiSha256, nil
}
