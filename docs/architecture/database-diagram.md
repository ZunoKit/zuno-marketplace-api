# Database Architecture Diagrams

## Complete Database Schema Overview

```mermaid
erDiagram
    auth_nonces {
        varchar nonce PK
        varchar account_id
        varchar domain
        varchar chain_id
        timestamp expires_at
        boolean used
    }
    
    sessions {
        uuid session_id PK
        uuid user_id FK
        varchar refresh_hash
        timestamp expires_at
        timestamp revoked_at
        jsonb collection_intent_context
    }
    
    login_events {
        uuid id PK
        uuid user_id FK
        varchar account_id
        varchar result
        timestamp event_timestamp
    }

    users {
        uuid user_id PK
        varchar status
        timestamp created_at
    }
    
    profiles {
        uuid user_id PK
        varchar username "UNIQUE"
        varchar display_name
        text bio
        text avatar_url
    }
    
    user_preferences {
        uuid user_id PK
        varchar theme
        varchar privacy_level
        boolean email_notifications
    }
    
    user_stats {
        uuid user_id PK
        integer collections_count
        integer items_count
        numeric volume_sold
        integer followers_count
    }
    
    user_follows {
        uuid follower_id PK
        uuid following_id PK
        timestamp created_at
    }

    wallet_links {
        uuid wallet_id PK
        uuid user_id FK
        varchar address
        varchar chain_id
        boolean is_primary
        varchar wallet_type
    }
    
    wallet_activity {
        uuid id PK
        uuid wallet_id FK
        varchar action
        jsonb metadata
    }
    
    wallet_verifications {
        uuid id PK
        uuid wallet_id FK
        varchar status
        jsonb verification_data
    }

    collections {
        uuid id PK
        text slug "UNIQUE"
        text name
        text chain_id
        text contract_address
        text creator
        text floor_price
        text volume_traded
    }
    
    tokens {
        uuid id PK
        uuid collection_id FK
        text token_number
        text owner_address
        text image_url
        boolean burned
    }
    
    traits {
        uuid id PK
        uuid collection_id FK
        text name
        text value_type
    }
    
    trait_values {
        uuid id PK
        uuid trait_id FK
        text value_string
        numeric rarity_score
    }
    
    token_trait_links {
        uuid token_id PK
        uuid trait_id PK
        uuid trait_value_id PK
    }
    
    marketplaces {
        uuid id PK
        text name "UNIQUE"
    }
    
    listings {
        uuid id PK
        uuid token_id FK
        uuid marketplace_id FK
        numeric price_native
        boolean is_active
    }
    
    offers {
        uuid id PK
        uuid token_id FK
        uuid marketplace_id FK
        numeric price_native
        text from_address
    }
    
    sales {
        uuid id PK
        uuid token_id FK
        uuid marketplace_id FK
        numeric price_native
        timestamp occurred_at
    }
    
    activities {
        uuid id PK
        uuid token_id FK
        text activity_type
        numeric price_native
        timestamp activity_timestamp
    }
    
    collection_stats {
        uuid collection_id PK
        integer items_count
        integer owners_count
        numeric floor_price_native
    }
    
    token_balances {
        text chain_id
        text contract
        text token_id
        text owner
        numeric quantity
    }
    
    ownership_transfers {
        text chain_id
        text contract
        text token_id
        text from_addr
        text to_addr
        text tx_hash
    }

    chains {
        integer id PK
        text caip2 "UNIQUE"
        integer chain_numeric "UNIQUE"
        text name
        boolean enabled
    }
    
    chain_endpoints {
        integer id PK
        integer chain_id FK
        text url
        integer priority
        boolean active
    }
    
    abi_blobs {
        varchar sha256 PK
        text standard
        jsonb abi_json
        text s3_key
    }
    
    chain_contracts {
        bigint id PK
        integer chain_id FK
        text address
        varchar abi_sha256 FK
        text standard
    }

    tx_intents {
        uuid intent_id PK
        text kind
        text chain_id
        text tx_hash
        text status
        uuid created_by FK
        varchar auth_session_id
    }
    
    session_intent_audit {
        uuid audit_id PK
        varchar session_id
        uuid intent_id FK
        uuid user_id FK
    }

    indexer_checkpoints {
        text chain_id PK
        bigint last_block
        text last_block_hash
        timestamp updated_at
    }

    users ||--|| profiles : has
    users ||--|| user_preferences : has
    users ||--|| user_stats : has
    users ||--o{ user_follows : follower
    users ||--o{ user_follows : following
    users ||--o{ wallet_links : owns
    users ||--o{ tx_intents : creates
    
    sessions }o--|| users : authenticates
    login_events }o--|| users : logs
    sessions ||--o{ session_intent_audit : correlates
    
    wallet_links ||--o{ wallet_activity : tracks
    wallet_links ||--o{ wallet_verifications : verifies
    
    collections ||--o{ tokens : contains
    collections ||--o{ traits : defines
    collections ||--|| collection_stats : has_stats
    tokens ||--o{ listings : listed_on
    tokens ||--o{ offers : receives
    tokens ||--o{ sales : sold_in
    tokens ||--o{ activities : generates
    tokens ||--o{ token_trait_links : has_traits
    traits ||--o{ trait_values : has_values
    trait_values ||--o{ token_trait_links : linked_to
    marketplaces ||--o{ listings : hosts
    marketplaces ||--o{ offers : processes
    marketplaces ||--o{ sales : records
    
    chains ||--o{ chain_endpoints : has_endpoints
    chains ||--o{ chain_contracts : deploys
    abi_blobs ||--o{ chain_contracts : used_by
    
    tx_intents ||--o{ session_intent_audit : audited
    session_intent_audit }o--|| users : references
```

## System Overview

```mermaid
graph TB
    subgraph "Frontend"
        WEB[Web App]
        MOBILE[Mobile App]
    end

    subgraph "API Gateway"
        GQL[GraphQL Gateway :8081]
    end

    subgraph "Core Services"
        AUTH[Auth Service :50051]
        USER[User Service :50052]
        WALLET[Wallet Service :50053]
        ORCH[Orchestrator Service :50054]
        MEDIA[Media Service :50055]
        CHAIN[Chain Registry :50056]
        CATALOG[Catalog Service :50057]
        INDEX[Indexer Service :50058]
    end

    subgraph "Data Layer"
        PG[(PostgreSQL)]
        MONGO[(MongoDB)]
        REDIS[(Redis)]
        MQ[RabbitMQ]
    end

    WEB --> GQL
    MOBILE --> GQL
    
    GQL --> AUTH
    GQL --> USER
    GQL --> WALLET
    GQL --> ORCH
    GQL --> MEDIA
    GQL --> CHAIN
    GQL --> CATALOG
    
    AUTH --> PG
    USER --> PG
    WALLET --> PG
    ORCH --> PG
    CHAIN --> PG
    CATALOG --> PG
    INDEX --> PG
    
    MEDIA --> MONGO
    INDEX --> MONGO
    
    AUTH --> REDIS
    USER --> REDIS
    
    INDEX --> MQ
    CATALOG --> MQ
```

## Auth Service Schema

```mermaid
erDiagram
    auth_nonces {
        varchar(64) nonce PK
        varchar(42) account_id
        varchar(255) domain
        varchar(32) chain_id
        timestamptz issued_at
        timestamptz expires_at
        boolean used
        timestamptz used_at
        timestamptz created_at
    }

    sessions {
        uuid session_id PK
        uuid user_id FK
        uuid device_id
        varchar(128) refresh_hash
        inet ip_address
        text user_agent
        timestamptz created_at
        timestamptz expires_at
        timestamptz revoked_at
        timestamptz last_used_at
        jsonb collection_intent_context
    }

    login_events {
        uuid id PK
        uuid user_id FK
        varchar(42) account_id
        inet ip_address
        text user_agent
        varchar(32) result
        text error_message
        varchar(32) chain_id
        varchar(255) domain
        timestamptz timestamp
    }

    sessions ||--o{ login_events : "logs"
    auth_nonces ||--o{ login_events : "validates"
```

## User Service Schema

```mermaid
erDiagram
    users {
        uuid user_id PK
        varchar(32) status
        timestamptz created_at
        timestamptz updated_at
    }

    profiles {
        uuid user_id PK
        varchar(30) username "UNIQUE"
        varchar(50) display_name
        text avatar_url
        text banner_url
        text bio
        varchar(10) locale
        varchar(50) timezone
        jsonb socials_json
        timestamptz updated_at
    }

    user_preferences {
        uuid user_id PK
        boolean email_notifications
        boolean push_notifications
        boolean marketing_emails
        varchar(10) language
        varchar(10) currency
        varchar(20) theme
        varchar(20) privacy_level
        boolean show_activity
        timestamptz updated_at
    }

    user_stats {
        uuid user_id PK
        integer collections_count
        integer items_count
        integer listings_count
        integer sales_count
        integer purchases_count
        numeric volume_sold
        numeric volume_purchased
        integer followers_count
        integer following_count
        timestamptz updated_at
    }

    user_follows {
        uuid follower_id PK
        uuid following_id PK
        timestamptz created_at
    }

    users ||--|| profiles : "has"
    users ||--|| user_preferences : "has"
    users ||--|| user_stats : "has"
    users ||--o{ user_follows : "follower"
    users ||--o{ user_follows : "following"
```

## Wallet Service Schema

```mermaid
erDiagram
    wallet_links {
        uuid wallet_id PK
        uuid user_id FK
        varchar(255) account_id
        varchar(42) address
        varchar(32) chain_id
        boolean is_primary
        varchar(20) type
        varchar(50) connector
        varchar(100) label
        timestamptz verified_at
        timestamptz created_at
        timestamptz updated_at
    }

    wallet_activity {
        uuid id PK
        uuid wallet_id FK
        uuid user_id
        varchar(50) action
        jsonb metadata
        inet ip_address
        text user_agent
        timestamptz created_at
    }

    wallet_verifications {
        uuid id PK
        uuid wallet_id FK
        varchar(50) verification_type
        jsonb verification_data
        varchar(20) status
        timestamptz verified_at
        timestamptz expires_at
        timestamptz created_at
    }

    wallet_links ||--o{ wallet_activity : "logs"
    wallet_links ||--o{ wallet_verifications : "verifies"
```

## Catalog Service Schema (Core Tables)

```mermaid
erDiagram
    collections {
        uuid id PK
        text slug "UNIQUE"
        text name
        text description
        text chain_id
        text contract_address
        text creator
        text owner
        text collection_type
        text max_supply
        text total_supply
        integer royalty_percentage
        text mint_price
        text floor_price
        text volume_traded
        boolean is_verified
        timestamptz created_at
    }

    tokens {
        uuid id PK
        uuid collection_id FK
        text chain_id
        text contract_address
        text token_number
        text token_standard
        text name
        text image_url
        text metadata_url
        text owner_address
        boolean burned
        timestamptz minted_at
    }

    traits {
        uuid id PK
        uuid collection_id FK
        text name
        text normalized_name
        text value_type
        text display_type
        integer sort_order
    }

    trait_values {
        uuid id PK
        uuid trait_id FK
        text value_string
        numeric value_number
        text normalized_value
        integer occurrences
        numeric rarity_score
    }

    token_trait_links {
        uuid token_id PK
        uuid trait_id PK
        uuid trait_value_id PK
    }

    collections ||--o{ tokens : "contains"
    collections ||--o{ traits : "defines"
    traits ||--o{ trait_values : "has"
    tokens ||--o{ token_trait_links : "has"
    traits ||--o{ token_trait_links : "links"
    trait_values ||--o{ token_trait_links : "values"
```

## Catalog Service Schema (Marketplace)

```mermaid
erDiagram
    marketplaces {
        uuid id PK
        text name "UNIQUE"
    }

    listings {
        uuid id PK
        uuid token_id FK
        uuid marketplace_id FK
        numeric price_native
        text currency_symbol
        boolean is_active
        text seller_address
        timestamptz listed_at
        timestamptz expires_at
        text tx_hash
    }

    offers {
        uuid id PK
        uuid token_id FK
        uuid marketplace_id FK
        numeric price_native
        text currency_symbol
        text from_address
        timestamptz created_at
        timestamptz expires_at
        text tx_hash
    }

    sales {
        uuid id PK
        uuid token_id FK
        uuid marketplace_id FK
        numeric price_native
        text currency_symbol
        text tx_hash
        timestamptz occurred_at
    }

    activities {
        uuid id PK
        uuid token_id FK
        text type
        text from_address
        text to_address
        numeric price_native
        text currency_symbol
        text tx_hash
        timestamptz timestamp
    }

    tokens ||--o{ listings : "listed"
    tokens ||--o{ offers : "receives"
    tokens ||--o{ sales : "sold"
    tokens ||--o{ activities : "tracks"
    marketplaces ||--o{ listings : "hosts"
    marketplaces ||--o{ offers : "handles"
    marketplaces ||--o{ sales : "processes"
```

## Chain Registry Schema

```mermaid
erDiagram
    chains {
        integer id PK
        text caip2 "UNIQUE"
        integer chain_numeric "UNIQUE"
        text name
        text native_symbol
        integer decimals
        text explorer_url
        boolean enabled
        jsonb features_json
    }

    chain_endpoints {
        integer id PK
        integer chain_id FK
        text url
        integer priority
        integer weight
        text auth_type
        integer rate_limit
        boolean active
    }

    abi_blobs {
        varchar sha256 PK
        integer size_bytes
        text source
        text compiler
        text contract_name
        text standard
        jsonb abi_json
        text s3_key
        timestamptz created_at
    }

    chain_contracts {
        bigint id PK
        integer chain_id FK
        text name
        text address
        integer start_block
        varchar abi_sha256 FK
        text impl_address
        text standard
        timestamptz first_seen_at
    }

    chain_gas_policy {
        integer chain_id PK
        numeric max_fee_gwei
        numeric priority_fee_gwei
        numeric multiplier
        numeric last_observed_base_fee_gwei
        timestamptz updated_at
    }

    chains ||--o{ chain_endpoints : "has"
    chains ||--o{ chain_contracts : "deploys"
    chains ||--|| chain_gas_policy : "configures"
    abi_blobs ||--o{ chain_contracts : "uses"
```

## Orchestrator Service Schema

```mermaid
erDiagram
    tx_intents {
        uuid intent_id PK
        text kind
        text chain_id
        text preview_address
        text tx_hash
        text status
        uuid created_by
        jsonb req_payload_json
        text error
        varchar(255) auth_session_id
        timestamptz deadline_at
        timestamptz created_at
        timestamptz updated_at
    }

    session_intent_audit {
        uuid audit_id PK
        varchar(255) session_id
        uuid intent_id FK
        uuid user_id
        timestamptz correlation_timestamp
        jsonb audit_data
        timestamptz created_at
    }

    tx_intents ||--o{ session_intent_audit : "audits"
```

## Cross-Service Relationships

```mermaid
graph LR
    subgraph "User Domain"
        U[users.user_id]
        P[profiles.user_id]
    end

    subgraph "Auth Domain"
        S[sessions.user_id]
        L[login_events.user_id]
    end

    subgraph "Wallet Domain"
        W[wallet_links.user_id]
        WA[wallet_activity.user_id]
    end

    subgraph "Catalog Domain"
        COL[collections.creator/owner]
        TOK[tokens.owner_address]
    end

    subgraph "Orchestrator Domain"
        TI[tx_intents.created_by]
        SI[session_intent_audit.user_id]
    end

    U --> P
    U --> S
    U --> L
    U --> W
    U --> WA
    U --> TI
    U --> SI
    
    W --> COL
    W --> TOK
    S --> SI
```

## Data Flow for Collection Creation

```mermaid
sequenceDiagram
    participant User
    participant GraphQL
    participant Auth
    participant Orchestrator
    participant Chain Registry
    participant Catalog
    participant Indexer
    participant Blockchain

    User->>GraphQL: Create Collection
    GraphQL->>Auth: Validate Session
    Auth-->>GraphQL: Session Valid
    
    GraphQL->>Orchestrator: Create Intent
    Orchestrator->>Chain Registry: Get Chain Config
    Chain Registry-->>Orchestrator: RPC Endpoints
    
    Orchestrator->>Blockchain: Deploy Contract
    Blockchain-->>Orchestrator: TX Hash
    
    Orchestrator->>Catalog: Store Pending Collection
    
    Indexer->>Blockchain: Monitor TX
    Blockchain-->>Indexer: TX Confirmed
    
    Indexer->>Catalog: Update Collection Status
    Indexer->>Orchestrator: Update Intent Status
    
    Orchestrator-->>GraphQL: Collection Created
    GraphQL-->>User: Success Response
```

## MongoDB Collections Structure

```mermaid
graph TD
    subgraph "Media Service MongoDB"
        MU[media_uploads]
        MU --> |contains| MU1[file_id]
        MU --> |contains| MU2[user_id]
        MU --> |contains| MU3[file_type]
        MU --> |contains| MU4[ipfs_hash]
        MU --> |contains| MU5[status]
        
        PJ[processing_jobs]
        PJ --> |contains| PJ1[job_id]
        PJ --> |contains| PJ2[file_id]
        PJ --> |contains| PJ3[operations]
        PJ --> |contains| PJ4[status]
    end

    subgraph "Indexer Service MongoDB"
        RE[raw_events]
        RE --> |contains| RE1[block_number]
        RE --> |contains| RE2[tx_hash]
        RE --> |contains| RE3[event_type]
        RE --> |contains| RE4[event_data]
        
        PQ[processing_queue]
        PQ --> |contains| PQ1[event_id]
        PQ --> |contains| PQ2[priority]
        PQ --> |contains| PQ3[retry_count]
        PQ --> |contains| PQ4[status]
    end
```

## Service Port Mapping

```mermaid
graph LR
    subgraph "gRPC Services"
        AUTH_P[Auth :50051]
        USER_P[User :50052]
        WALLET_P[Wallet :50053]
        ORCH_P[Orchestrator :50054]
        MEDIA_P[Media :50055]
        CHAIN_P[Chain Registry :50056]
        CATALOG_P[Catalog :50057]
        INDEXER_P[Indexer :50058]
    end

    subgraph "HTTP Services"
        GRAPHQL[GraphQL Gateway :8081]
    end

    subgraph "Infrastructure"
        PG_P[PostgreSQL :5432]
        REDIS_P[Redis :6379]
        RABBIT_P[RabbitMQ :5672/:15672]
        MONGO_P[MongoDB :27017]
    end

    GRAPHQL --> AUTH_P
    GRAPHQL --> USER_P
    GRAPHQL --> WALLET_P
    GRAPHQL --> ORCH_P
    GRAPHQL --> MEDIA_P
    GRAPHQL --> CHAIN_P
    GRAPHQL --> CATALOG_P
```

## Database Size Estimates

```mermaid
pie title "Estimated Storage Distribution"
    "Catalog (Collections/Tokens)" : 45
    "Media (IPFS/Images)" : 25
    "Chain Registry (ABIs)" : 10
    "User Data" : 8
    "Activities/Events" : 7
    "Auth/Sessions" : 3
    "Other" : 2
```
