# Database Schema Documentation

## Overview

The NFT Marketplace uses a microservice architecture with PostgreSQL as the primary database and MongoDB for specific services. Each service manages its own database schema with clear boundaries.

## Service Database Schemas

### 1. Auth Service (`auth_service`)

Manages authentication, sessions, and nonces for Sign-In with Ethereum (SIWE).

#### Tables:

##### `auth_nonces`
- **Purpose**: Store one-time nonces for SIWE authentication
- **Key Fields**:
  - `nonce` (varchar(64), PK): Unique hex string
  - `account_id` (varchar(42)): Ethereum address (lowercase 0x...)
  - `domain` (varchar(255)): Domain requesting authentication
  - `chain_id` (varchar(32)): CAIP-2 identifier (e.g., eip155:1)
  - `expires_at` (timestamptz): TTL (max 10 minutes)
  - `used` (boolean): Whether nonce has been consumed
- **Constraints**: 
  - Address must be lowercase 0x + 40 hex chars
  - Expiry must be > issued_at and <= 10 minutes

##### `sessions`
- **Purpose**: User sessions after successful SIWE verification
- **Key Fields**:
  - `session_id` (uuid, PK): Unique session identifier
  - `user_id` (uuid): References user service
  - `refresh_hash` (varchar(128)): Hash of refresh token
  - `ip_address` (inet): Client IP
  - `expires_at` (timestamptz): Session expiration
  - `revoked_at` (timestamptz): When session was revoked
  - `collection_intent_context` (jsonb): Optional collection creation context
- **Indexes**: On user_id, expires_at, refresh_hash (unique)

##### `login_events`
- **Purpose**: Audit log of all authentication attempts
- **Key Fields**:
  - `id` (uuid, PK): Event identifier
  - `user_id` (uuid): User if authenticated
  - `account_id` (varchar(42)): Wallet address used
  - `result` (varchar(32)): success/failed/invalid_signature/etc
  - `error_message` (text): Details if failed
- **Valid Results**: success, failed, invalid_signature, invalid_nonce, expired_nonce, invalid_message, rate_limited

### 2. User Service (`user_service`)

Manages user profiles, preferences, and relationships.

#### Tables:

##### `users`
- **Purpose**: Core user accounts
- **Key Fields**:
  - `user_id` (uuid, PK): Unique user identifier
  - `status` (varchar(32)): active/banned/deleted/suspended
  - `created_at`, `updated_at` (timestamptz)
- **Triggers**: Auto-creates related records in profiles, preferences, stats

##### `profiles`
- **Purpose**: User profile information
- **Key Fields**:
  - `user_id` (uuid, PK, FK→users): One-to-one with users
  - `username` (varchar(30), unique): Alphanumeric + underscore, 3-30 chars
  - `display_name` (varchar(50)): Display name (max 50 chars)
  - `bio` (text): User bio (max 500 chars)
  - `avatar_url`, `banner_url` (text): Media URLs
  - `socials_json` (jsonb): Social media links

##### `user_preferences`
- **Purpose**: User settings and preferences
- **Key Fields**:
  - `user_id` (uuid, PK, FK→users)
  - `email_notifications`, `push_notifications`, `marketing_emails` (boolean)
  - `theme` (varchar(20)): light/dark/auto
  - `privacy_level` (varchar(20)): public/private/friends
  - `currency` (varchar(10)): Default currency

##### `user_stats`
- **Purpose**: Aggregated user statistics
- **Key Fields**:
  - `user_id` (uuid, PK, FK→users)
  - `collections_count`, `items_count`, `listings_count` (integer)
  - `sales_count`, `purchases_count` (integer)
  - `volume_sold`, `volume_purchased` (numeric(20,8))
  - `followers_count`, `following_count` (integer)
- **Constraints**: All counts must be >= 0

##### `user_follows`
- **Purpose**: User follow relationships
- **Key Fields**:
  - `follower_id`, `following_id` (uuid, FK→users): Composite PK
  - `created_at` (timestamptz)
- **Constraints**: No self-follows
- **Triggers**: Updates follower/following counts in user_stats

### 3. Wallet Service (`wallet_service`)

Manages wallet connections and verification.

#### Tables:

##### `wallet_links`
- **Purpose**: User wallet connections
- **Key Fields**:
  - `wallet_id` (uuid, PK): Unique wallet link
  - `user_id` (uuid): User owning this wallet
  - `address` (varchar(42)): Ethereum address (lowercase 0x...)
  - `chain_id` (varchar(32)): CAIP-2 format (e.g., eip155:1)
  - `is_primary` (boolean): Primary wallet flag
  - `type` (varchar(20)): eoa/contract/multisig/smart_account
  - `connector` (varchar(50)): metamask/walletconnect/etc
- **Unique Constraints**:
  - One address per user per chain
  - Only one primary wallet per user

##### `wallet_activity`
- **Purpose**: Audit log of wallet actions
- **Key Fields**:
  - `id` (uuid, PK): Activity identifier
  - `wallet_id` (uuid, FK→wallet_links)
  - `action` (varchar(50)): linked/unlinked/set_primary/verified/updated
  - `metadata` (jsonb): Additional context
  - `ip_address` (inet), `user_agent` (text)

##### `wallet_verifications`
- **Purpose**: Wallet ownership verification records
- **Key Fields**:
  - `id` (uuid, PK): Verification identifier
  - `wallet_id` (uuid, FK→wallet_links)
  - `verification_type` (varchar(50)): signature/transaction/etc
  - `status` (varchar(20)): pending/verified/failed/expired
  - `verification_data` (jsonb): Verification details

### 4. Catalog Service (`catalog_service`)

Core NFT catalog with collections, tokens, and marketplace data.

#### Tables:

##### `collections`
- **Purpose**: NFT collections registry
- **Key Fields**:
  - `id` (uuid, PK): Collection identifier
  - `slug` (text, unique): URL-friendly identifier
  - `name`, `description` (text): Basic info
  - `chain_id` (text): Blockchain identifier
  - `contract_address` (text): Smart contract address
  - `creator`, `owner` (text): Addresses
  - `collection_type` (text): Type of collection
  - `max_supply`, `total_supply` (text): Supply limits
  - Minting config: `mint_price`, `mint_limit_per_wallet`, `mint_start_time`, etc.
  - Social links: `discord_url`, `twitter_url`, `instagram_url`, etc.
  - Market data: `floor_price`, `volume_traded`
- **Unique**: (chain_id, contract_address)

##### `tokens`
- **Purpose**: Individual NFT tokens
- **Key Fields**:
  - `id` (uuid, PK): Token identifier
  - `collection_id` (uuid, FK→collections)
  - `chain_id`, `contract_address` (text)
  - `token_number` (text): Token ID (string for big ints)
  - `token_standard` (text): ERC721/ERC1155/etc
  - `name`, `image_url`, `metadata_url` (text)
  - `owner_address` (text): Current owner
  - `burned` (boolean): Burn status
- **Unique**: (collection_id, token_number)

##### `traits` & `trait_values`
- **Purpose**: NFT attributes and rarity
- **traits**: Trait definitions per collection
- **trait_values**: Possible values with occurrences and rarity scores
- **token_trait_links**: Links tokens to their trait values

##### `listings`, `offers`, `sales`
- **Purpose**: Marketplace activity
- **Common Fields**:
  - Price data: `price_native`, `currency_symbol`
  - `marketplace_id` (FK→marketplaces)
  - `tx_hash`: Transaction hash
- **listings**: Active marketplace listings
- **offers**: Purchase offers
- **sales**: Completed sales history

##### `activities`
- **Purpose**: Activity feed for tokens
- **Key Fields**:
  - `type` (text): listed/offer/sale/transfer/mint/burn
  - `token_id` (uuid, FK→tokens)
  - Price and transaction data
  - `timestamp` (timestamptz): When it occurred

### 5. Chain Registry Service (`chain_registry_service`)

Manages blockchain configurations and smart contract ABIs.

#### Tables:

##### `chains`
- **Purpose**: Supported blockchain networks
- **Key Fields**:
  - `id` (serial, PK)
  - `caip2` (caip2_chain, unique): CAIP-2 identifier (e.g., eip155:1)
  - `chain_numeric` (integer, unique): Chain ID number
  - `name`, `native_symbol` (text): Chain info
  - `enabled` (boolean): Whether chain is active

##### `chain_endpoints`
- **Purpose**: RPC endpoints per chain
- **Key Fields**:
  - `chain_id` (FK→chains)
  - `url` (text): RPC endpoint
  - `priority`, `weight` (integer): Load balancing
  - `rate_limit` (integer): Rate limiting

##### `abi_blobs`
- **Purpose**: Smart contract ABI storage
- **Key Fields**:
  - `sha256` (char(64), PK): Content-addressed storage
  - `standard` (text): erc721/erc1155/custom/proxy
  - `abi_json` (jsonb): Full ABI JSON
  - `s3_key` (text): Storage location

##### `chain_contracts`
- **Purpose**: Contract registry per chain
- **Key Fields**:
  - `chain_id` (FK→chains)
  - `address` (evm_address): Contract address
  - `abi_sha256` (FK→abi_blobs): Link to ABI
  - `standard` (text): Contract standard

### 6. Orchestrator Service (`orchestrator_service`)

Manages transaction intents and workflows.

#### Tables:

##### `tx_intents`
- **Purpose**: Transaction intent tracking
- **Key Fields**:
  - `intent_id` (uuid, PK): Intent identifier
  - `kind` (text): collection/mint/etc
  - `chain_id` (caip2_chain): Target chain
  - `tx_hash` (evm_tx_hash): Final transaction
  - `status` (text): pending/ready/failed/expired
  - `auth_session_id` (varchar(255)): Session correlation
  - `req_payload_json` (jsonb): Request data

##### `session_intent_audit`
- **Purpose**: Audit trail for session-intent correlation
- **Key Fields**:
  - `session_id`, `intent_id`, `user_id`: Correlation data
  - `audit_data` (jsonb): Additional audit information

### 7. Indexer Service (`indexer_service`)

Tracks blockchain indexing progress.

#### Tables:

##### `indexer_checkpoints`
- **Purpose**: Indexer progress per chain
- **Key Fields**:
  - `chain_id` (text, PK): CAIP-2 Chain ID
  - `last_block` (bigint): Last processed block height
  - `last_block_hash` (text): Hash of last block
  - `updated_at` (timestamptz): Last update time

## MongoDB Collections

### Media Service
- **media_uploads**: File upload metadata
- **processing_jobs**: Media processing queue

### Indexer Service  
- **raw_events**: Raw blockchain events
- **processing_queue**: Event processing queue

## Database Conventions

### Common Patterns
1. **UUID Primary Keys**: Most tables use UUID for distributed generation
2. **Timestamps**: `created_at`, `updated_at` fields are standard
3. **Soft Deletes**: Status fields instead of hard deletes
4. **JSONB Fields**: For flexible, schema-less data
5. **Lowercase Addresses**: All Ethereum addresses stored as lowercase

### Indexes Strategy
1. **Primary Keys**: Automatic B-tree indexes
2. **Foreign Keys**: Indexed for join performance
3. **Unique Constraints**: Enforce business rules
4. **Partial Indexes**: For filtered queries (e.g., active records only)
5. **Covering Indexes**: Include columns to avoid table lookups

### Data Types
- **Addresses**: `varchar(42)` for Ethereum addresses
- **Chain IDs**: CAIP-2 format strings
- **Amounts**: `numeric` or `text` for big numbers
- **Timestamps**: Always `timestamptz` for timezone awareness

## Migration Strategy

### Principles
1. **Forward-Only**: No down migrations (removed all down.sql files)
2. **Idempotent**: Migrations use IF NOT EXISTS
3. **Non-Breaking**: Add columns as nullable, migrate data, then add constraints
4. **Atomic**: Each migration in a transaction

### Best Practices
1. Always backup before migrations
2. Test migrations in staging first
3. Monitor migration performance
4. Keep migrations small and focused
5. Document breaking changes
