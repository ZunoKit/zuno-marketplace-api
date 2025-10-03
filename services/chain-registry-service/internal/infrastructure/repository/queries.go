package repository

// SQL queries for chain registry operations
const (
	// Chain queries
	QueryGetChainInfo = `
		SELECT name, native_symbol, decimals 
		FROM chains 
		WHERE caip2 = $1 AND enabled = true
	`

	// Contract queries
	QueryGetContracts = `
		SELECT name, address, start_block, verified_at, standard, impl_address, abi_sha256 
		FROM chain_contracts 
		WHERE chain_id = (SELECT id FROM chains WHERE caip2 = $1)
		ORDER BY name, address
	`

	QueryGetContractMeta = `
		SELECT name, address, start_block, verified_at, standard, impl_address, abi_sha256
		FROM chain_contracts 
		WHERE chain_id = (SELECT id FROM chains WHERE caip2 = $1) AND address = $2
	`

	QueryGetProxyContract = `
		SELECT impl_address, abi_sha256, standard
		FROM chain_contracts 
		WHERE chain_id = (SELECT id FROM chains WHERE caip2 = $1) AND address = $2
	`

	// Gas policy queries
	QueryGetGasPolicy = `
		SELECT max_fee_gwei, priority_fee_gwei, multiplier, last_observed_base_fee_gwei, updated_at
		FROM chain_gas_policy 
		WHERE chain_id = (SELECT id FROM chains WHERE caip2 = $1)
	`

	// RPC endpoint queries
	QueryGetRpcEndpoints = `
		SELECT url, priority, weight, auth_type, rate_limit, active
		FROM chain_endpoints 
		WHERE chain_id = (SELECT id FROM chains WHERE caip2 = $1) AND active = true
		ORDER BY priority, weight DESC
	`

	// ABI blob queries
	QueryGetAbiBlobMetadata = `
		SELECT s3_key, size_bytes 
		FROM abi_blobs 
		WHERE sha256 = $1
	`

	QueryGetAbiBlobJson = `
		SELECT abi_json::text 
		FROM abi_blobs 
		WHERE sha256 = $1
	`
)
