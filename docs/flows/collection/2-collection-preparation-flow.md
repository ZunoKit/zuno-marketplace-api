# 2. Collection Preparation Flow

## Overview

This document describes the collection preparation flow, covering validation, contract configuration, and intent creation.

## Sequence Diagram

```mermaid
sequenceDiagram
  autonumber
  participant FE as FE
  participant GQL as GraphQL Gateway
  participant COL as CollectionSvc
  participant CHREG as ChainRegistry
  participant RREG as "Redis (registry cache w/ version)"
  participant PGR as "Postgres (chain_registry_db)"
  participant PGO as "Postgres (orchestrator_db)"
  participant RINT as "Redis (intent status)"

  Note over FE: Media uploaded, user fills collection details

  FE->>GQL: mutation prepareCreateCollection(input)
  GQL->>COL: Prepare(input) (gRPC)
  COL->>CHREG: GetContracts(chainId)
  CHREG->>RREG: GET cache:chains:chainId:version
  alt cache miss
    CHREG->>PGR: SELECT contracts/policy
    CHREG->>RREG: SET cache:chains:chainId:version EX 60
  end
  COL->>PGO: INSERT tx_intents(kind='collection', chain_id, created_by, deadline_at) RETURNING intent_id
  COL-->>GQL: intentId, txRequest{to=factory,data,value}, previewAddress?
  COL->>RINT: SET intent:status:intentId "pending" EX 21600
```

## Key Components

### Collection Validation
- Name and symbol uniqueness check
- Metadata completeness validation
- Royalty percentage limits (0-10%)
- Media reference verification

### Chain Registry Integration
- Fetches factory contract addresses
- Retrieves gas policies and limits
- Validates chain support
- Caches configuration for performance

#### Chain Registry Caching Flow
```mermaid
sequenceDiagram
  autonumber
  participant COL as CollectionSvc
  participant REG as ChainRegistrySvc
  participant R as Redis (cache)
  participant PGR as Postgres (chain_registry_db)
  participant MQ as RabbitMQ (optional)

  COL->>REG: GetContracts(chainId)
  REG->>R: GET cache:chains:{chainId}:version
  alt cache hit
    REG->>R: HGETALL cache:chains:{chainId}:contracts / gas / endpoints
    R-->>REG: cached config
    REG-->>COL: contracts + requiredConfirmations + version
  else cache miss
    REG->>PGR: SELECT * FROM CHAINS / CHAIN_CONTRACTS / CHAIN_GAS_POLICY / CHAIN_ENDPOINTS
    REG->>R: HMSET cache:chains:{chainId}:contracts/gas/endpoints
    REG->>R: SET cache:chains:{chainId}:version EX 60
    REG-->>COL: contracts + requiredConfirmations + version
  end

  note over MQ,REG: (Optional) khi admin cập nhật DB, publish<br/>chainregistry.version.bump {chainId, version}
  MQ-->>REG: consume bump
  REG->>R: DEL cache:chains:{chainId}:*   # invalidate
```

### Contract Configuration
- **Factory Address**: Contract that deploys collections
- **Implementation**: Base contract template
- **Gas Policy**: Dynamic gas pricing strategy
- **Feature Support**: Chain-specific capabilities

## Input Validation

### Required Fields
```json
{
  "name": "Collection Name",
  "symbol": "SYMBOL",
  "description": "Collection description",
  "chainId": "eip155:1",
  "logoCid": "QmHash...",
  "bannerCid": "QmHash...",
  "royaltyBps": 250,
  "royaltyReceiver": "0x..."
}
```

### Optional Fields
```json
{
  "website": "https://example.com",
  "twitter": "@username",
  "discord": "https://discord.gg/...",
  "maxSupply": 10000,
  "mintPrice": "0.1",
  "category": "art|gaming|music|..."
}
```

## Contract Data Preparation

### Factory Call Data
```solidity
function createCollection(
    string memory name,
    string memory symbol,
    string memory baseURI,
    uint96 royaltyBps,
    address royaltyReceiver,
    bytes memory initData
) external returns (address collection)
```

### Init Data Structure
- Collection metadata URI
- Mint configuration
- Access control settings
- Feature flags

## Intent Management

### Intent Record
```sql
INSERT INTO tx_intents (
    intent_id,
    kind,
    chain_id,
    created_by,
    req_payload_json,
    deadline_at,
    status
) VALUES (
    uuid_generate_v4(),
    'collection',
    'eip155:1',
    'user_id',
    '{"name":"...","symbol":"..."}',
    NOW() + INTERVAL '6 hours',
    'pending'
)
```

### Status Tracking
- **pending**: Intent created, awaiting transaction
- **broadcast**: Transaction sent to blockchain
- **confirmed**: Factory contract called successfully
- **indexed**: Collection detected by indexer
- **ready**: Collection available in catalog

## Preview Address

### Deterministic Deployment
- Calculate collection address before deployment
- Use CREATE2 opcode for predictable addresses
- Enable frontend preview and linking
- Validate address uniqueness

### Address Calculation
```solidity
bytes32 salt = keccak256(abi.encodePacked(creator, nonce));
address predicted = Clones.predictDeterministicAddress(
    implementation,
    salt,
    factory
);
```

## Error Scenarios

### Validation Failures
- Invalid chain ID or unsupported network
- Duplicate collection name/symbol
- Invalid royalty configuration
- Missing required media assets

### Registry Issues
- Chain configuration not found
- Factory contract not deployed
- Gas policy outdated
- Network connectivity problems

### Intent Creation Failures
- Database connection issues
- Intent deadline conflicts
- User permission errors
- Storage quota exceeded