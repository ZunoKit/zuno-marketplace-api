# NFT Minting Overview

## Overview

This document provides a high-level overview of the NFT minting system and references to detailed flow documentation.

## NFT Minting Flows

The NFT minting system consists of 5 main flows:

### 1. Intent Creation Flow
**File**: [1-intent-creation-flow.md](./1-intent-creation-flow.md)

Mint preparation and intent management:
- Collection selection and mint parameters
- Chain registry configuration lookup
- Intent creation with tracking ID
- Transaction request preparation
- Redis status caching

### 2. Transaction Broadcast Flow
**File**: [2-transaction-broadcast-flow.md](./2-transaction-broadcast-flow.md)

Wallet interaction and blockchain submission:
- Wallet integration (MetaMask, WalletConnect)
- Transaction signing and broadcasting
- Gas price management and estimation
- Transaction hash tracking
- Error handling and retry logic

### 3. Blockchain Confirmation Flow
**File**: [3-blockchain-confirmation-flow.md](./3-blockchain-confirmation-flow.md)

Smart contract execution and event emission:
- Contract mint function execution
- ERC-721/ERC-1155 token creation
- Transfer event emission
- Confirmation depth requirements
- Chain reorganization handling

### 4. Event Processing Flow
**File**: [4-event-processing-flow.md](./4-event-processing-flow.md)

Blockchain event indexing and catalog updates:
- Event detection and raw storage
- Checkpoint management
- Message queue publishing
- Metadata fetching and normalization
- Catalog database updates

### 5. Realtime Notification Flow
**File**: [5-realtime-notification-flow.md](./5-realtime-notification-flow.md)

Intent completion and user notifications:
- Domain event consumption
- Intent-to-transaction linking
- Status updates in Redis
- WebSocket notifications
- Frontend subscription handling

## Architecture Components

### Services
- **Mint Service**: NFT minting orchestration
- **Chain Registry**: Contract and gas policy management
- **Indexer Service**: Blockchain event monitoring
- **Catalog Service**: NFT metadata and ownership
- **Subscription Worker**: Real-time notifications

### Databases
- **PostgreSQL**: Intents, catalog data, indexer checkpoints
- **MongoDB**: Raw events, metadata documents
- **Redis**: Intent status, chain registry cache
- **RabbitMQ**: Event processing pipeline

### Blockchain Integration
- **JSON-RPC**: Blockchain node communication
- **Smart Contracts**: ERC-721/ERC-1155 collections
- **Event Monitoring**: Transfer event detection
- **Gas Management**: Dynamic pricing strategies

## Token Standards

### ERC-721 (Non-Fungible Tokens)
```solidity
function mint(address to, uint256 tokenId) external {
    _mint(to, tokenId);
    // Emits: Transfer(address(0), to, tokenId)
}
```

**Features**:
- Unique token IDs
- Single owner per token
- Individual metadata URIs
- Suitable for unique digital assets

### ERC-1155 (Multi-Token Standard)
```solidity
function mint(address to, uint256 id, uint256 amount, bytes memory data) external {
    _mint(to, id, amount, data);
    // Emits: TransferSingle(operator, address(0), to, id, amount)
}

function mintBatch(address to, uint256[] memory ids, uint256[] memory amounts, bytes memory data) external {
    _mintBatch(to, ids, amounts, data);
    // Emits: TransferBatch(operator, address(0), to, ids, amounts)
}
```

**Features**:
- Fungible and non-fungible tokens
- Batch operations for efficiency
- Shared metadata for token types
- Suitable for gaming items and collectibles

## GraphQL Schema

```graphql
type NFT {
  id: ID!
  collection: Collection!
  tokenId: String!
  owner: User!
  metadata: NFTMetadata!
  standard: TokenStandard!
  mintedAt: DateTime!
}

type NFTMetadata {
  name: String
  description: String
  image: String
  attributes: [Attribute!]!
}

type Attribute {
  traitType: String!
  value: String!
  displayType: String
}

input PrepareMintInput {
  collectionAddress: Address!
  chainId: ChainId!
  to: Address!
  tokenId: String
  amount: Int
  metadata: String
}

type PrepareMintPayload {
  intentId: String!
  txRequest: TransactionRequest!
}

type MintStatus {
  status: IntentStatus!
  contract: Address
  tokenIds: [String!]
  txHash: String
  chainId: ChainId
  error: String
}

type Mutation {
  prepareMint721(input: PrepareMintInput!): PrepareMintPayload!
  prepareMint1155(input: PrepareMintInput!): PrepareMintPayload!
  trackMintTx(intentId: String!, txHash: String!): Boolean!
}

type Subscription {
  onMintStatus(intentId: String!): MintStatus!
}
```

## Intent-Based Architecture

### Intent Status Lifecycle
```
pending → broadcast → confirming → ready
   ↓           ↓           ↓
 failed     failed     failed
```

### Status Descriptions
- **pending**: Intent created, awaiting transaction
- **broadcast**: Transaction sent to blockchain
- **confirming**: Transaction confirmed, processing events
- **ready**: Minting complete, NFT available
- **failed**: Process failed at any stage

## Metadata Processing

### Token URI Resolution
```javascript
// For ERC-721
const tokenURI = await contract.tokenURI(tokenId)

// For ERC-1155
const uri = await contract.uri(tokenId)
```

### Metadata Standards
- **OpenSea**: Standard attribute format
- **IPFS**: Decentralized metadata storage
- **Normalization**: Consistent data structure
- **Media Processing**: Image and video handling

## Multi-Chain Support

### Chain Configuration
- Contract addresses per network
- Gas policies and confirmation depths
- RPC endpoint management
- Feature flags and capabilities

### Supported Networks
- Ethereum Mainnet
- Polygon
- Binance Smart Chain
- Arbitrum
- Optimism

## Real-Time Features

### WebSocket Subscriptions
```graphql
subscription onMintStatus($intentId: String!) {
  onMintStatus(intentId: $intentId) {
    status
    contract
    tokenIds
    txHash
    chainId
    error
  }
}
```

### Progress Tracking
- Real-time status updates
- Transaction confirmation progress
- Token ID revelation on completion
- Error reporting with details
- Automatic UI updates

## Security & Performance

### Security Features
- Intent-based architecture prevents front-running
- Signature validation and replay protection
- Access control for minting permissions
- Rate limiting on mint endpoints
- Transaction monitoring and fraud detection

### Performance Optimizations
- Redis caching for fast status lookup
- Event batching and parallel processing
- Connection pooling for databases
- CDN for metadata delivery
- Horizontal scaling capabilities

### Error Handling
- Comprehensive retry logic
- Graceful degradation on failures
- User-friendly error messages
- Automatic recovery mechanisms
- Monitoring and alerting systems