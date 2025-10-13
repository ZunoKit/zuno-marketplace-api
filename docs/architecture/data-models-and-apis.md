# Data Models and APIs

### Database Architecture

**PostgreSQL** (separate schemas per service):
- `auth`: Sessions, nonces, login events, device fingerprints
- `user`: User profiles and account data
- `wallets`: Multi-wallet support, approvals, history
- `chain_registry`: Chain configs, endpoints, contracts, gas policies
- `orchestrator`: Transaction intents and orchestration
- `catalog`: Collections, NFTs, marketplace data, statistics
- `indexer`: Blockchain checkpoints and processing state

**MongoDB**:
- `events.raw`: Raw blockchain events (indexer-service)
- `metadata.docs`: NFT metadata cache (media-service)
- `media.assets`: Media assets and variants (media-service)

**Redis**:
- Sessions: `session:blacklist:*`, `siwe:nonce:*`
- Caching: `cache:*` (catalog-service), `cache:chains:*`
- Intent status: `intent:status:*`
- Wallet approvals: `wallet:approvals:cache:*`

### API Specifications

- **GraphQL Schema**: `services/graphql-gateway/graphql/schema.graphqls`
- **gRPC Services**: See `proto/*.proto` files for exact definitions
- **WebSocket**: Real-time subscriptions via GraphQL gateway
- **REST**: Health checks and metrics on `/health` and `/metrics`
