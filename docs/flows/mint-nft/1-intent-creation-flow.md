# 1. Intent Creation Flow

## Overview

This document describes the intent creation flow for NFT minting, covering the preparation phase before blockchain transaction.

## Sequence Diagram

```mermaid
sequenceDiagram
  autonumber
  actor U as Creator/Owner
  participant FE as FE
  participant GQL as GraphQL Gateway
  participant MINT as MintSvc
  participant CHREG as ChainRegistry
  participant RREG as "Redis (registry cache:version)"
  participant PGR as "Postgres (chain_registry_db)"
  participant PGO as "Postgres (orchestrator_db)"
  participant RINT as "Redis (intent status)"

  U->>FE: Chọn collection + nhập thông số mint
  FE->>GQL: mutation prepareMint721(...) / prepareMint1155(...)
  GQL->>MINT: PrepareMint (gRPC)
  MINT->>CHREG: GetContracts/Policy(chainId)
  CHREG->>RREG: GET cache:chains:chainId:version
  alt cache miss
    CHREG->>PGR: SELECT contracts/policy
    CHREG->>RREG: SET cache:chains:chainId:version EX 60
  end
  MINT->>PGO: INSERT tx_intents(kind='mint', chain_id, created_by, deadline_at) RETURNING intent_id
  MINT->>RINT: SET intent:status:intentId "pending" EX 21600
  MINT-->>GQL: intentId, txRequest{to=collection,data,value}
  GQL-->>FE: intentId + txRequest
```

## Key Components

### Intent Management
- Creates unique intent ID for tracking
- Stores intent in PostgreSQL with deadline
- Sets Redis status for fast lookup
- Returns transaction request data

### Chain Registry
- Provides contract addresses and gas policies
- Uses Redis cache for performance
- Falls back to PostgreSQL on cache miss
- 60-second cache TTL for fresh data

### Transaction Request
- Prepares calldata for mint function
- Calculates gas estimates
- Returns structured transaction object
- Ready for wallet signing

## Data Flow

1. **User Input**: Collection selection and mint parameters
2. **Chain Config**: Fetch contracts and policies from registry
3. **Intent Storage**: Create tracking record in database
4. **Status Cache**: Set initial pending status in Redis
5. **Response**: Return intent ID and transaction request

## Error Handling

- Chain registry failures fallback to database
- Invalid parameters return validation errors
- Intent creation failures are logged and reported
- Cache misses don't block the flow