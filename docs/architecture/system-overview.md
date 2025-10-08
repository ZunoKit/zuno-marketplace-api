# System Architecture Overview

## Documentation Index

- [Database Schema](./database-schema.md) - Detailed database schema documentation
- [Database Diagrams](./database-diagram.md) - Visual database architecture with Mermaid diagrams

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