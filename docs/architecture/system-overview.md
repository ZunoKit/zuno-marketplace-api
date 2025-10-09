# System Architecture Overview

## Documentation Index

- [Database Schema](./database-schema.md) - Detailed database schema documentation
- [Database Diagrams](./database-diagram.md) - Visual database architecture with Mermaid diagrams
- [Production Readiness](../production-readiness.md) - Production features and deployment guide
- [API Documentation](../api/README.md) - Complete API reference and examples

## Microservices Architecture

```mermaid
flowchart LR
  FE["Frontend (Next.js)"]
  WS[["GraphQL WS"]]
  GQL["GraphQL Gateway / BFF"]

  AUTH["Auth Svc (SIWE / Sessions)"]
  USER["User Svc (Users / Profiles)"]
  WALLET["Wallet Svc (Addresses / Approvals)"]

  MQ[("RabbitMQ")]
  DBA[("Postgres Auth")]
  DBU[("Postgres User")]
  DBW[("Postgres Wallets")]
  REDIS[("Redis")]

  FE -- "HTTP GraphQL" --> GQL
  WS --- GQL

  GQL <-- "gRPC" --> AUTH
  GQL <-- "gRPC" --> USER
  GQL <-- "gRPC" --> WALLET

  AUTH --- DBA
  AUTH --- REDIS
  USER --- DBU
  WALLET --- DBW

  AUTH --"publish auth.user.verified"--> MQ
  MQ --"consume"--> USER
  AUTH -. "EnsureUser (sync, tuỳ chọn)" .-> USER
  AUTH -. "UpsertLink" .-> WALLET
```

## Service Database Mapping

```mermaid
flowchart LR
  GQL[GraphQL Gateway/BFF]

  subgraph AUTH["Auth Service"]
    A1[(PG.auth\nauth_nonces,\nsessions, login_events)]
    A2[(Redis\nsiwe:nonce:*, session:blacklist:*)]
  end

  subgraph WALLET["Wallet Service"]
    W1[(PG.wallets\nwallets, approvals, approvals_history)]
    W2[(Redis\nwallet:approvals:cache:*)]
  end

  subgraph USER["User/Profile Service"]
    U1[(PG.user\nusers, profiles)]
  end

  subgraph REG["Chain Registry Service"]
    R1[(PG.chain_registry\nchains, chain_endpoints,\nchain_contracts, chain_gas_policy)]
    R2[(Redis\ncache:chains:chainId:version)]
  end

  subgraph ORCH["Orchestrator (Collection/Mint)"]
    O1[(PG.orchestrator\ntx_intents)]
    O2[(Redis\nintent:status:intentId)]
  end

  subgraph IDX["Indexer"]
    I1[(Mongo.events\nevents.raw)]
    I2[(PG.indexer\nindexer_checkpoints)]
  end

  subgraph CAT["Catalog Service"]
    C1[(PG.catalog\ncollections,*bindings,*roles,*mint_config,\n*nfts,*token_balances,*ownership_transfers,*nft_flags,\ntraits, trait_values, token_trait_links,\nmarketplaces, listings, offers, sales,\norders, order_fills,\ncollection_stats, token_rarity, rarity_scores, trait_value_floor,\nsync_state, processed_events)]
  end

  subgraph MEDIA["Media/Metadata Upload"]
    M1[(Mongo.metadata\nmetadata.docs)]
    M2[(Mongo.media\nmedia.assets, media.variants)]
  end

  subgraph CACHE["Read Cache"]
    R3[(Redis\ncache:read:*)]
  end

  GQL --> AUTH
  GQL --> WALLET
  GQL --> USER
  GQL --> REG
  GQL --> ORCH
  GQL --> IDX
  GQL --> CAT
  GQL --> MEDIA
  CAT --> CACHE
```

## Message Queue Architecture

```rabbitmq
Exchange

auth.events (topic, durable) → sự kiện của Auth

wallets.events (topic, durable) → sự kiện của Wallet

dlx.events (topic, durable) → dead-letter exchange dùng chung

Queues

subs.auth.logged_in ← bind auth.events với key user.logged_in

subs.wallets.linked ← bind wallets.events với key wallet.linked

Mỗi queue gắn DLX + TTL retry
```

## Production Security Architecture

### mTLS Communication
```mermaid
flowchart LR
  subgraph "External Zone"
    CLIENT[Web Client]
  end
  
  subgraph "DMZ"
    GW[GraphQL Gateway]
  end
  
  subgraph "Internal Zone"
    AUTH[Auth Service]
    USER[User Service]
    WALLET[Wallet Service]
  end
  
  CLIENT --"HTTPS"--> GW
  GW <--"mTLS/gRPC"--> AUTH
  GW <--"mTLS/gRPC"--> USER
  GW <--"mTLS/gRPC"--> WALLET
```

### Security Features
- **Authentication**: SIWE with refresh token rotation
- **Authorization**: Role-based access control with GraphQL directives
- **Encryption**: mTLS for all internal communication
- **Rate Limiting**: Token bucket algorithm at GraphQL layer
- **Session Security**: Device fingerprinting and tracking
- **Input Validation**: Schema validation and sanitization

## Performance Architecture

### Caching Strategy
```mermaid
flowchart TD
  REQUEST[Client Request]
  GATEWAY[GraphQL Gateway]
  CACHE{Cache Hit?}
  REDIS[(Redis Cache)]
  SERVICE[Microservice]
  DB[(Database)]
  
  REQUEST --> GATEWAY
  GATEWAY --> CACHE
  CACHE -->|Yes| REDIS
  REDIS --> GATEWAY
  CACHE -->|No| SERVICE
  SERVICE --> DB
  SERVICE --> REDIS
```

### Connection Pooling
- **PostgreSQL**: Service-specific pool configurations
- **Redis**: Cluster-aware connection pooling
- **gRPC**: Connection reuse with keep-alive

### Circuit Breakers
- Automatic failure detection
- Service isolation during outages
- Gradual recovery with half-open state

## Monitoring & Observability

### Metrics Collection
```mermaid
flowchart LR
  SERVICES[Services]
  METRICS[Prometheus Metrics]
  GRAFANA[Grafana]
  ALERTS[Alert Manager]
  
  SERVICES --"expose /metrics"--> METRICS
  METRICS --> GRAFANA
  METRICS --> ALERTS
```

### Logging Pipeline
- **Structured Logging**: JSON format with zerolog
- **Log Aggregation**: Centralized log collection
- **Correlation IDs**: Request tracing across services
- **Audit Trail**: Security and compliance logging

### Health Checks
- `/health` - Liveness probe
- `/ready` - Readiness probe
- `/metrics` - Prometheus metrics endpoint