# NFT Minting Process

## Overview

This document describes the NFT minting process supporting both ERC-721 and ERC-1155 standards.

## Minting Sequence Diagram

```mermaid
sequenceDiagram
  autonumber
  actor U as Creator/Owner
  participant FE as FE
  participant WAL as Wallet
  participant GQL as GraphQL Gateway
  participant MINT as MintSvc
  participant CHREG as ChainRegistry
  participant RREG as "Redis (registry cache:version)"
  participant PGR as "Postgres (chain_registry_db)"
  participant PGO as "Postgres (orchestrator_db)"
  participant RINT as "Redis (intent status)"
  participant CH as "JSON-RPC"
  participant NFT as "Collection Contract"
  participant IDX as Indexer
  participant MGE as "Mongo (events.raw)"
  participant MQ as RabbitMQ
  participant CAT as Catalog
  participant PGC as "Postgres (catalog_db)"
  participant MG as "Mongo (metadata.docs)"
  participant SUB as SubsWorker

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

  FE->>WAL: eth_sendTransaction(txRequest)
  WAL->>CH: broadcast
  CH-->>WAL: txHash
  FE->>GQL: mutation trackMintTx(intentId, chainId, txHash, contract)
  GQL->>MINT: TrackMintTx(...)
  MINT->>PGO: UPDATE tx_intents SET tx_hash=?, status='pending' WHERE intent_id=?
  MINT->>RINT: SET intent:status:intentId "pending" EX 21600

  CH-->>NFT: mint(...)
  alt ERC-721
    NFT-->>CH: Transfer(0x0 -> to, tokenId)
  else ERC-1155
    NFT-->>CH: TransferSingle/TransferBatch(...)
  end

  CH-->>IDX: logs (N confirmations)
  IDX->>MGE: INSERT events.raw (unique by chainId,txHash,logIndex)
  IDX->>PGC: UPDATE indexer_checkpoints
  IDX-->>MQ: publish mints.events minted.eip155-chainId {schema,v1,event_id,txHash,contract,tokenIds,...}

  MQ-->>CAT: consume minted.*
  CAT->>PGC: INSERT processed_events(event_id) ON CONFLICT DO NOTHING
  alt first time
    CAT->>CH: tokenURI/uri(...) (eth_call) if needed
    CAT->>MG: UPSERT metadata.docs(chainId,contract,tokenId,normalized,media,...)
    CAT->>PGC: UPSERT nfts(chain_id,contract,token_id,owner,token_uri,metadata_doc,standard,updated_at)
    CAT-->>MQ: publish mints.domain upserted.chainId.contract.tokenId {schema,v1,txHash,tokenIds,...}
  else duplicate
    CAT-->>MQ: ack
  end

  MQ-->>SUB: consume mints.domain upserted.*
  SUB->>PGO: SELECT intent_id FROM tx_intents WHERE chain_id=? AND tx_hash=? AND kind='mint'
  alt found
    SUB->>RINT: SET intent:status:intentId "ready" EX 21600
    SUB-->>GQL: push WS onMintStatus(intentId,{status:"ready",contract,tokenIds,txHash,chainId})
  else not found
    SUB-->>SUB: retry later
  end

  FE->>GQL: subscription onMintStatus(intentId)
  GQL-->>FE: realtime update
```

## Minting Standards

### ERC-721 (Non-Fungible Tokens)
- Each token has a unique ID
- Single owner per token
- Suitable for unique digital assets

### ERC-1155 (Multi-Token Standard)
- Supports both fungible and non-fungible tokens
- Batch operations for efficiency
- Suitable for gaming items, collectibles

## Key Features

### Intent-Based Architecture
- Creates intent before blockchain transaction
- Tracks transaction lifecycle
- Provides real-time status updates

### Metadata Handling
- Fetches metadata from tokenURI
- Normalizes data structure
- Stores in MongoDB for fast access

### Multi-Chain Support
- Chain-specific contract configurations
- Dynamic gas policy management
- Cross-chain deployment capability

### Event Processing
- Monitors blockchain events
- Ensures idempotent processing
- Handles transaction confirmations

### Real-time Updates
- WebSocket subscriptions
- Intent status tracking
- Live minting progress