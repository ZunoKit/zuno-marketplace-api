# NFT Marketplace API Documentation

## Overview

The NFT Marketplace API is a production-ready, multi-chain NFT marketplace backend built with GraphQL Gateway and gRPC microservices architecture.

## Table of Contents

1. [Getting Started](#getting-started)
2. [Authentication](#authentication)
3. [GraphQL API](#graphql-api)
4. [Rate Limiting](#rate-limiting)
5. [Error Handling](#error-handling)
6. [WebSocket Subscriptions](#websocket-subscriptions)
7. [Security](#security)

## Getting Started

### Base URL

```
Production: https://api.marketplace.example.com/graphql
Staging: https://staging-api.marketplace.example.com/graphql
Local: http://localhost:8081/graphql
```

### Headers

Required headers for all requests:

```http
Content-Type: application/json
Authorization: Bearer <jwt-token>
X-Chain-Id: eip155:1  # Target blockchain
```

## Authentication

### SIWE (Sign-In with Ethereum)

#### 1. Request Nonce

```graphql
mutation GetNonce {
  signInSiwe {
    message
    nonce
  }
}
```

#### 2. Sign Message

Sign the message with your wallet (client-side).

#### 3. Verify Signature

```graphql
mutation VerifySiwe($signature: String!, $message: String!, $nonce: String!) {
  verifySiwe(signature: $signature, message: $message, nonce: $nonce) {
    accessToken
    refreshToken
    user {
      id
      wallets
    }
  }
}
```

### Session Refresh

```graphql
mutation RefreshSession {
  refreshSession {
    accessToken
    refreshToken
  }
}
```

## GraphQL API

### Collections

#### Create Collection

```graphql
mutation CreateCollection($input: CreateCollectionInput!) {
  prepareCreateCollection(input: $input) {
    intentId
    unsignedTransaction {
      to
      data
      value
      gasLimit
    }
  }
}
```

#### Submit Transaction

```graphql
mutation SubmitCollectionTx($intentId: String!, $txHash: String!) {
  submitCollectionTx(intentId: $intentId, txHash: $txHash) {
    success
    collection {
      id
      contractAddress
      name
      symbol
    }
  }
}
```

### NFT Minting

#### Prepare Mint

```graphql
mutation PrepareMint($input: MintNFTInput!) {
  prepareMint(input: $input) {
    intentId
    unsignedTransaction {
      to
      data
      value
      gasLimit
    }
  }
}
```

#### Submit Mint Transaction

```graphql
mutation SubmitMintTx($intentId: String!, $txHash: String!) {
  submitMintTx(intentId: $intentId, txHash: $txHash) {
    success
    nft {
      id
      tokenId
      owner
      metadata {
        name
        description
        image
      }
    }
  }
}
```

### Marketplace Operations

#### Create Listing

```graphql
mutation CreateListing($input: CreateListingInput!) {
  createListing(input: $input) {
    id
    nft {
      id
      tokenId
    }
    price
    currency
    status
  }
}
```

#### Make Offer

```graphql
mutation MakeOffer($input: MakeOfferInput!) {
  makeOffer(input: $input) {
    id
    amount
    currency
    expiresAt
    status
  }
}
```

### Queries

#### Get User Profile

```graphql
query GetProfile {
  me {
    id
    username
    wallets
    collections {
      id
      name
      contractAddress
    }
    nfts {
      id
      tokenId
      metadata {
        name
        image
      }
    }
  }
}
```

#### Search NFTs

```graphql
query SearchNFTs($filter: NFTFilter!, $pagination: PaginationInput) {
  searchNFTs(filter: $filter, pagination: $pagination) {
    items {
      id
      tokenId
      owner
      metadata {
        name
        description
        image
      }
      listings {
        price
        currency
      }
    }
    totalCount
    hasMore
  }
}
```

## Rate Limiting

### Default Limits

| Endpoint | Limit | Window |
|----------|-------|--------|
| signInSiwe | 10 | 1 minute |
| verifySiwe | 5 | 1 minute |
| createCollection | 5 | 1 hour |
| prepareMint | 20 | 1 minute |
| searchNFTs | 60 | 1 minute |

### Headers

Rate limit information in response headers:

```http
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1672531200
```

## Error Handling

### Error Response Format

```json
{
  "errors": [
    {
      "message": "Unauthorized",
      "extensions": {
        "code": "UNAUTHENTICATED",
        "statusCode": 401
      }
    }
  ]
}
```

### Common Error Codes

| Code | Status | Description |
|------|--------|-------------|
| UNAUTHENTICATED | 401 | Missing or invalid authentication |
| FORBIDDEN | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource not found |
| VALIDATION_ERROR | 400 | Invalid input |
| RATE_LIMITED | 429 | Too many requests |
| INTERNAL_ERROR | 500 | Server error |

## WebSocket Subscriptions

### Connection

```javascript
const ws = new WebSocket('wss://api.marketplace.example.com/graphql');

// Send connection init with auth
ws.send(JSON.stringify({
  type: 'connection_init',
  payload: {
    authorization: 'Bearer <jwt-token>'
  }
}));
```

### Subscribe to Events

```graphql
subscription OnNFTMinted($collectionId: ID!) {
  nftMinted(collectionId: $collectionId) {
    id
    tokenId
    owner
    metadata {
      name
      image
    }
  }
}
```

## Security

### Best Practices

1. **Always use HTTPS** in production
2. **Implement request signing** for sensitive operations
3. **Validate addresses** on both client and server
4. **Use rate limiting** to prevent abuse
5. **Enable 2FA** for high-value accounts
6. **Monitor for suspicious activity**

### Security Headers

The API sets the following security headers:

```http
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Strict-Transport-Security: max-age=31536000; includeSubDomains
Content-Security-Policy: default-src 'self'
```

### Token Rotation

- Access tokens expire after 1 hour
- Refresh tokens expire after 7 days
- Tokens are rotated on each refresh
- Old tokens are invalidated immediately

## SDK Examples

### JavaScript/TypeScript

```typescript
import { MarketplaceSDK } from '@zuno/marketplace-sdk';

const sdk = new MarketplaceSDK({
  endpoint: 'https://api.marketplace.example.com/graphql',
  chainId: 'eip155:1'
});

// Authenticate
await sdk.auth.signIn(signer);

// Create collection
const { intentId, transaction } = await sdk.collections.prepare({
  name: 'My Collection',
  symbol: 'MYC',
  type: 'ERC721'
});

// Sign and send transaction
const txHash = await signer.sendTransaction(transaction);

// Submit transaction
const collection = await sdk.collections.submit(intentId, txHash);
```

### Python

```python
from zuno_marketplace import MarketplaceClient

client = MarketplaceClient(
    endpoint="https://api.marketplace.example.com/graphql",
    chain_id="eip155:1"
)

# Authenticate
client.auth.sign_in(wallet_address, signature)

# Search NFTs
nfts = client.nfts.search(
    collection_id="...",
    owner="0x...",
    limit=10
)
```

## Changelog

### v1.0.0 (Current)

- Initial production release
- SIWE authentication
- Multi-chain support
- NFT minting and trading
- WebSocket subscriptions
- Rate limiting and security features

---

For more information, visit our [Developer Portal](https://developers.marketplace.example.com)
