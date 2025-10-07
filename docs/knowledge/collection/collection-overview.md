# Collection Overview

## Overview

This document provides a high-level overview of the NFT collection creation system and references to detailed flow documentation.

## Collection Creation Flows

The collection creation system consists of 5 main flows:

### 1. Media Upload Flow
**File**: [1-media-upload-flow.md](./1-media-upload-flow.md)

Media processing and storage including:
- Synchronous and asynchronous upload patterns
- S3/IPFS storage integration
- File deduplication by SHA256
- Pinata IPFS pinning service
- Real-time upload progress

### 2. Collection Preparation Flow
**File**: [2-collection-preparation-flow.md](./2-collection-preparation-flow.md)

Collection configuration and validation:
- Input validation and metadata verification
- Chain registry integration with caching
- Factory contract configuration
- Intent creation and tracking
- Deterministic address prediction

### 3. Contract Deployment Flow
**File**: [3-contract-deployment-flow.md](./3-contract-deployment-flow.md)

Blockchain contract deployment:
- Wallet integration and transaction signing
- Factory contract execution
- EIP-1167 minimal proxy deployment
- Gas optimization and management
- Security and access control setup

### 4. Collection Indexing Flow
**File**: [4-collection-indexing-flow.md](./4-collection-indexing-flow.md)

Blockchain event processing:
- Event detection with confirmation requirements
- Raw event storage and checkpoint management
- Message queue publishing
- Catalog processing and metadata enrichment
- Idempotent event handling

### 5. Collection Completion Flow
**File**: [5-collection-completion-flow.md](./5-collection-completion-flow.md)

Intent resolution and notifications:
- Domain event consumption
- Intent status updates
- Real-time WebSocket notifications
- Collection availability in marketplace
- Post-creation management features

## Architecture Components

### Services
- **Collection Service**: Collection deployment and management
- **Media Service**: File upload and IPFS integration
- **Chain Registry**: Multi-chain contract configuration
- **Indexer Service**: Blockchain event monitoring
- **Catalog Service**: NFT metadata and marketplace data

### Databases
- **PostgreSQL**: Collections, intents, indexer state
- **MongoDB**: Media assets, metadata, raw events
- **Redis**: Chain registry cache, intent status
- **RabbitMQ**: Event processing pipeline

### Storage
- **S3/MinIO**: File storage and CDN delivery
- **IPFS**: Decentralized metadata storage
- **Pinata**: IPFS pinning service

## Contract Architecture

### Factory Pattern
```solidity
contract CollectionFactory {
  function createCollection(
    string memory name,
    string memory symbol,
    string memory baseURI,
    uint96 royaltyBps,
    address royaltyReceiver,
    bytes memory initData
  ) external returns (address collection);
}
```

### Collection Implementation
- **EIP-1167**: Minimal proxy for gas efficiency
- **EIP-2981**: NFT royalty standard
- **EIP-165**: Interface detection
- **Access Control**: Role-based permissions
- **Upgradeable**: UUPS proxy pattern

## GraphQL Schema

```graphql
type Collection {
  id: ID!
  name: String!
  symbol: String!
  description: String
  contractAddress: Address!
  chainId: ChainId!
  creator: User!
  totalSupply: Int!
  royaltyBps: Int!
  metadata: CollectionMetadata!
}

type CollectionMetadata {
  image: String
  banner: String
  website: String
  social: SocialLinks
}

input CreateCollectionInput {
  name: String!
  symbol: String!
  description: String
  chainId: ChainId!
  logoCid: String!
  bannerCid: String
  royaltyBps: Int!
  royaltyReceiver: Address!
}

type CreateCollectionPayload {
  intentId: String!
  txRequest: TransactionRequest!
  previewAddress: Address
}

type Mutation {
  uploadMedia(files: [Upload!]!): [MediaAsset!]!
  prepareCreateCollection(input: CreateCollectionInput!): CreateCollectionPayload!
  trackCollectionTx(intentId: String!, txHash: String!): Boolean!
}

type Subscription {
  onCollectionStatus(intentId: String!): CollectionStatus!
  onMediaPinned(assetId: String!): MediaAsset!
}
```

## Intent-Based Architecture

### Intent Lifecycle
```
pending → broadcast → confirmed → indexed → ready
   ↓           ↓           ↓         ↓
 failed     failed     failed    failed
```

### Status Tracking
- **PostgreSQL**: Persistent intent storage
- **Redis**: Fast status lookup with TTL
- **WebSocket**: Real-time status updates
- **Message Queue**: Event-driven processing

## Multi-Chain Support

### Chain Registry
- Contract addresses per chain
- Gas policies and confirmation requirements
- RPC endpoint management
- Feature flag configuration

### Supported Networks
- Ethereum Mainnet
- Polygon
- Binance Smart Chain
- Arbitrum
- Optimism

## Media Processing

### File Types
- **Images**: PNG, JPG, GIF, WebP, SVG
- **Videos**: MP4, WebM, MOV
- **Audio**: MP3, WAV, OGG

### Processing Pipeline
1. **Upload**: Multipart streaming to backend
2. **Storage**: S3 with SHA256 deduplication
3. **Processing**: Thumbnail generation and optimization
4. **IPFS**: Pinning for decentralized access
5. **CDN**: Fast global content delivery

## Security & Performance

### Security Features
- Input validation and sanitization
- Rate limiting on upload endpoints
- IPFS content verification
- Contract deployment security
- Access control and permissions

### Performance Optimizations
- Redis caching for chain configuration
- Event batching and processing
- CDN for media delivery
- Connection pooling for databases
- Horizontal scaling capabilities