# NFT Marketplace Architecture - Smart Contract Based

## 1.Overview System

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



```mermaid
sequenceDiagram
  autonumber
  actor U as User
  participant FE as FE (Next.js)
  participant WAL as Wallet
  participant GQL as GraphQL Gateway
  participant AUTH as Auth Svc (gRPC)
  participant USER as User Svc (gRPC)
  participant WALLET as Wallet Svc (gRPC)
  participant R as Redis (auth cache)
  participant PGA as Postgres (auth_db)
  participant DBU as Postgres (user_db)
  participant PGW as Postgres (wallet_db)
  participant MQ as RabbitMQ

  U->>FE: Click "Connect"
  FE->>WAL: request accounts + chainId
  FE->>GQL: signInSiwe(accountId, chainId, domain)
  GQL->>AUTH: GetNonce(...)

  Note over AUTH: Create one-time nonce (single-use, TTL)
  AUTH->>PGA: INSERT auth_nonces(...)
  AUTH->>R: SET siwe:nonce:{nonce} EX 300
  AUTH-->>GQL: {nonce}
  GQL-->>FE: {nonce}

  FE->>WAL: personal_sign(message)
  WAL-->>FE: signature
  FE->>GQL: verifySiwe(message, signature, accountId)
  GQL->>AUTH: VerifySiwe(...)

  Note over AUTH: Validate domain/chainId/nonce and recover (EOA/EIP-1271)
  AUTH->>R: GET siwe:nonce:{nonce}

  alt first time user
    AUTH->>USER: EnsureUser(accountId, address)
    USER->>DBU: UPSERT user & profile
    USER-->>AUTH: {user_id}
  else existing
    AUTH->>USER: EnsureUser(accountId, address)  # idempotent
    USER-->>AUTH: {user_id}
  end

  Note over AUTH: Create session & link wallet (idempotent)
  AUTH->>PGA: UPDATE auth_nonces SET used=true
  AUTH->>PGA: INSERT sessions(...)
  AUTH->>WALLET: UpsertLink(user_id, accountId, address, chainId, is_primary=true)
  WALLET->>PGW: INSERT ... ON CONFLICT DO NOTHING

  AUTH-->>GQL: {accessToken, refreshToken, userId, expiresAt}
  GQL-->>FE: session established

  AUTH->>MQ: publish auth.user_logged_in {...}
  WALLET->>MQ: publish wallet.linked {...}



```

```graphql
scalar Address
scalar ChainId
scalar Hex
scalar DateTime

type NoncePayload {
  nonce: String!
}

type AuthPayload {
  accessToken: String!
  refreshToken: String! # hoặc bỏ khỏi body và set cookie httpOnly
  expiresAt: DateTime!
  userId: ID!
}

input SignInSiweInput {
  accountId: String!
  chainId: ChainId!
  domain: String!
}

input VerifySiweInput {
  accountId: String!
  message: String!
  signature: Hex!
}

type Mutation {
  signInSiwe(input: SignInSiweInput!): NoncePayload!
  verifySiwe(input: VerifySiweInput!): AuthPayload!
}
```

```proto
syntax = "proto3";
package auth.v1;
option go_package = "github.com/yourorg/packages/proto-gen-go/auth/v1;authv1";

message GetNonceRequest { string account_id = 1; string chain_id = 2; string domain = 3; }
message GetNonceResponse { string nonce = 1; }

message VerifySiweRequest { string account_id = 1; string message = 2; string signature = 3; }
message VerifySiweResponse {
  string access_token  = 1;
  string refresh_token = 2;
  string expires_at    = 3;
  string user_id       = 4;
  string address       = 5;
  string chain_id      = 6;
}

service Auth {
  rpc GetNonce(GetNonceRequest) returns (GetNonceResponse);
  rpc VerifySiwe(VerifySiweRequest) returns (VerifySiweResponse);
}



```

```proto
syntax = "proto3";
package wallets.v1;
option go_package = "github.com/yourorg/packages/proto-gen-go/wallets/v1;walletsv1";

message UpsertLinkRequest {
  string user_id   = 1;
  string account_id= 2;
  string address   = 3; // lowercase 0x…
  string chain_id  = 4; // CAIP-2
  bool   is_primary= 5;
}
message UpsertLinkResponse {}

service Wallets {
  rpc UpsertLink (UpsertLinkRequest) returns (UpsertLinkResponse);
}


```

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

## 3. Create Collection

```mermaid
sequenceDiagram
  autonumber
  actor U as Creator
  participant FE as FE
  participant WAL as Wallet
  participant GQL as GraphQL Gateway
  participant MEDIA as MediaSvc
  participant MGM as "Mongo (media.assets)"
  participant OBJ as "S3/IPFS"
  participant COL as CollectionSvc
  participant RINT as "Redis (intent status)"
  participant PGO as "Postgres (orchestrator_db)"
  participant CHREG as ChainRegistry
  participant RREG as "Redis (registry cache w/ version)"
  participant PGR as "Postgres (chain_registry_db)"
  participant CH as "JSON-RPC"
  participant FAC as "Factory Contract"
  participant IDX as Indexer
  participant MGE as "Mongo (events.raw)"
  participant MQ as RabbitMQ
  participant CAT as Catalog
  participant PGC as "Postgres (catalog_db)"
  participant SUB as SubsWorker

  U->>FE: Open Create Collection

  %% Upload media
  FE->>GQL: mutation uploadMedia(files)
  GQL->>MEDIA: Upload (gRPC)
  MEDIA->>OBJ: putObject / pin CID
  MEDIA->>MGM: INSERT media.assets{cid,mime,bytes,variants,...}
  MEDIA-->>GQL: {logoCid,bannerCid,urls}
  GQL-->>FE: media refs

  %% Prepare calldata & intent (idempotent)
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

  FE->>WAL: eth_sendTransaction(txRequest)
  WAL->>CH: broadcast
  CH-->>WAL: txHash
  FE->>GQL: mutation trackCollectionTx(intentId, chainId, txHash, previewAddress?)
  GQL->>COL: TrackTx(...)
  COL->>PGO: UPDATE tx_intents SET tx_hash=?, status='pending' WHERE intent_id=?
  COL->>RINT: SET intent:status:intentId "pending" EX 21600

  CH-->>FAC: createCollection(...)
  FAC-->>CH: emit CollectionCreated(...)

  CH-->>IDX: logs (after N confirmations configurable)
  IDX->>MGE: INSERT events.raw (unique by chainId,txHash,logIndex)
  IDX->>PGC: UPDATE indexer_checkpoints
  IDX-->>MQ: publish collections.events created.eip155-chainId {schema,v1,event_id,txHash,contract,...}

  MQ-->>CAT: consume created.*
  CAT->>PGC: INSERT processed_events(event_id) ON CONFLICT DO NOTHING
  alt first time
    CAT->>PGC: UPSERT collections(..., royalty_bps, royalty_receiver, standard, ...)
    CAT-->>MQ: publish collections.domain upserted.chainId.contract {schema,v1,txHash,contract,...}
  else duplicate
    CAT-->>MQ: ack
  end

  MQ-->>SUB: consume collections.domain upserted.*
  SUB->>PGO: SELECT intent_id FROM tx_intents WHERE chain_id=? AND tx_hash=? AND kind='collection'
  alt found
    SUB->>RINT: SET intent:status:intentId "ready" EX 21600
    SUB-->>GQL: push WS onCollectionStatus(intentId,{status:"ready",address,chainId,txHash})
  else not found
    SUB-->>SUB: schedule retry
  end

  FE->>GQL: subscription onCollectionStatus(intentId)
  GQL-->>FE: realtime update


```

## 4. Mint NFT

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

## 5. Service Database Mapping

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

## 5. Database Schema

```mermaid
erDiagram
  %% ======================= AUTH SERVICE (Postgres) =======================
  AUTH_NONCES {
    string   nonce PK
    string   account_id
    string   domain
    string   chain_id
    datetime issued_at
    datetime expires_at
    boolean  used
  }

  SESSIONS {
    uuid     session_id PK
    uuid     user_id
    uuid     device_id
    string   refresh_hash
    string   ip
    string   ua
    datetime created_at
    datetime expires_at
    datetime revoked_at
    datetime last_used_at
  }

  LOGIN_EVENTS {
    uuid     id PK
    uuid     user_id
    string   account_id
    string   ip
    string   ua
    string   result
    datetime ts
  }

  SESSIONS ||..|| LOGIN_EVENTS : audit

  %% ======================= WALLET SERVICE (Postgres) =======================
  WALLETS {
    uuid     id PK
    uuid     user_id
    string   account_id
    string   address
    string   chain_id
    string   type
    string   connector
    boolean  is_primary
    string   label
    datetime verified_at
    datetime created_at
    datetime updated_at
    datetime last_seen_at
  }

  APPROVALS {
    uuid     wallet_id
    string   chain_id
    string   operator
    string   standard
    boolean  approved
    datetime approved_at
    datetime revoked_at
    string   tx_hash
    datetime updated_at
  }

  APPROVALS_HISTORY {
    uuid     id PK
    uuid     wallet_id
    string   chain_id
    string   operator
    string   standard
    boolean  approved
    string   tx_hash
    datetime at
  }

  WALLETS ||--o{ APPROVALS : has
  WALLETS ||--o{ APPROVALS_HISTORY : changes

  %% ======================= USER / PROFILE (Postgres) =======================
  USERS {
    uuid     id PK
    string   status
    datetime created_at
  }

  PROFILES {
    uuid     user_id PK
    string   username
    string   display_name
    string   avatar_url
    string   banner_url
    string   bio
    string   locale
    string   timezone
    string   socials_json
    datetime updated_at
  }

  USERS ||--|| PROFILES : owns
  USERS ||--o{ WALLETS : has

  %% ======================= CHAIN REGISTRY (Postgres) =======================
  CHAINS {
    int      id PK
    string   caip2
    int      chain_numeric
    string   name
    string   native_symbol
    int      decimals
    string   explorer_url
    boolean  enabled
    string   features_json
  }

  CHAIN_ENDPOINTS {
    int      id PK
    int      chain_id
    string   url
    int      priority
    int      weight
    string   auth_type
    int      rate_limit
    boolean  active
  }

  CHAIN_CONTRACTS {
    int      id PK
    int      chain_id
    string   name
    string   address
    int      start_block
    datetime verified_at
  }

  CHAIN_GAS_POLICY {
    int      chain_id PK
    float    max_fee_gwei
    float    priority_fee_gwei
    float    multiplier
    float    last_observed_base_fee_gwei
    datetime updated_at
  }

  CHAINS ||--o{ CHAIN_ENDPOINTS : has
  CHAINS ||--o{ CHAIN_CONTRACTS : has
  CHAINS ||--|| CHAIN_GAS_POLICY : has

  %% ======================= ORCHESTRATOR (Postgres) =======================
  TX_INTENTS {
    uuid     intent_id PK
    string   kind
    string   chain_id
    string   preview_address
    string   tx_hash
    string   status
    uuid     created_by
    string   req_payload_json
    string   error
    datetime deadline_at
    datetime created_at
    datetime updated_at
  }

  %% ======================= INDEXER (PG + Mongo) =======================
  INDEXER_CHECKPOINTS {
    string   chain_id PK
    int      last_block
    string   last_block_hash
    datetime updated_at
  }

  EVENTS_RAW {
    string   _id PK
    string   eventId
    string   chainId
    int      blockNumber
    string   blockHash
    string   txHash
    int      logIndex
    string   address
    string   topics_json
    string   data
    string   parsed_json
    datetime observedAt
    int      confirmations
  }

  %% ======================= CATALOG (Postgres) =======================
  COLLECTIONS {
    uuid     id PK
    string   slug
    string   name
    string   description
    string   category
    string   image_url
    string   banner_url
    string   website_url
    string   social_links_json
    boolean  is_verified
    boolean  is_hidden
    string   source
    int      total_supply
    int      royalty_bps
    string   royalty_receiver
    string   metadata_standard
    string   status
    datetime mint_start_date
    datetime mint_end_date
    int      total_minted
    int      max_supply
    string   mint_price_text
    int      deployed_block
    string   index_status
    string   tags_json
    string   settings_json
    datetime created_at
    datetime updated_at
  }

  COLLECTION_ROLES {
    string   chain_id
    string   address
    string   role
    string   account
    datetime granted_at
  }

  COLLECTION_BINDINGS {
    uuid     id PK
    uuid     collection_id
    string   chain_id
    string   family
    string   token_standard
    string   contract_address
    string   mint_authority
    string   inscription_id
    boolean  is_primary
  }

  COLLECTION_MINT_CONFIG {
    uuid     collection_id PK
    datetime start_date
    datetime end_date
    string   mint_price_text
  }

  TOKENS {
    uuid     id PK
    uuid     collection_id
    string   chain_id
    string   family
    string   contract_address
    string   mint_address
    string   inscription_id
    string   token_number
    string   token_standard
    int      supply
    boolean  burned
    string   name
    string   image_url
    string   metadata_url
    string   owner_address
    int      minted_block
    datetime minted_at
    datetime last_refresh_at
    string   metadata_doc
  }

  TRAITS {
    uuid     id PK
    uuid     collection_id
    string   name
    string   normalized_name
    string   value_type
    string   display_type
    string   unit
    int      sort_order
  }

  TRAIT_VALUES {
    uuid     id PK
    uuid     trait_id
    string   value_type
    string   value_string
    float    value_number
    bigint   value_epoch_seconds
    string   normalized_value
    int      occurrences
    float    frequency
    float    rarity_score
    float    max_value
    string   unit
  }

  TOKEN_TRAIT_LINKS {
    uuid     token_id
    uuid     trait_id
    uuid     trait_value_id
  }

  TOKEN_BALANCES {
    string   chain_id
    string   contract
    string   token_id
    string   owner
    numeric  quantity
    datetime updated_at
  }

  OWNERSHIP_TRANSFERS {
    string   chain_id
    string   contract
    string   token_id
    string   from_addr
    string   to_addr
    string   tx_hash
    int      log_index
    datetime at
  }

  NFT_FLAGS {
    string   chain_id
    string   contract
    string   token_id
    boolean  is_flagged
    boolean  is_spam
    boolean  is_frozen
    boolean  is_nsfw
    boolean  refreshable
    string   reason_json
    datetime updated_at
  }

  MARKETPLACES {
    uuid     id PK
    string   name
  }

  LISTINGS {
    uuid     id PK
    uuid     token_id
    uuid     marketplace_id
    numeric  price_native
    string   price_native_text
    string   currency_symbol
    boolean  is_active
    datetime listed_at
    datetime updated_at
    string   seller_address
    datetime expires_at
    string   url
    string   tx_hash
  }

  OFFERS {
    uuid     id PK
    uuid     token_id
    uuid     marketplace_id
    numeric  price_native
    string   price_native_text
    string   currency_symbol
    string   from_address
    datetime created_at
    datetime expires_at
    string   tx_hash
  }

  SALES {
    uuid     id PK
    uuid     token_id
    uuid     marketplace_id
    numeric  price_native
    string   price_native_text
    string   currency_symbol
    string   tx_hash
    datetime occurred_at
  }

  ORDERS {
    uuid     id PK
    uuid     token_id
    string   side
    string   maker
    string   taker
    numeric  price_native
    string   currency_symbol
    datetime start_at
    datetime end_at
    string   signature
    string   salt
    string   source_marketplace
    string   status
    datetime updated_at
  }

  ORDER_FILLS {
    uuid     id PK
    uuid     order_id
    string   tx_hash
    numeric  price_native
    datetime filled_at
  }

  COLLECTION_STATS {
    uuid     collection_id PK
    int      items_count
    int      owners_count
    numeric  floor_price_native
    string   floor_currency_symbol
    numeric  market_cap_est
    datetime last_updated_at
  }

  TOKEN_RARITY {
    uuid     token_id PK
    float    rarity_score_product
  }

  RARITY_SCORES {
    uuid     token_id
    string   method
    string   source
    float    score
    int      rank
    datetime updated_at
  }

  TRAIT_VALUE_FLOOR {
    uuid     trait_value_id PK
    numeric  floor_price_native
    datetime last_updated_at
  }

  ACTIVITIES {
    uuid     id PK
    uuid     token_id
    string   type
    string   from_address
    string   to_address
    numeric  price_native
    string   price_native_text
    string   currency_symbol
    string   tx_hash
    string   block_or_slot
    datetime timestamp
    string   marketplace
  }

  SYNC_STATE {
    uuid     id PK
    uuid     collection_id
    string   source
    string   cursor
    datetime last_run_at
    string   note
  }

  PROCESSED_EVENTS {
    string   event_id PK
    int      event_version
    string   chain_id
    string   block_hash
    int      log_index
    datetime processed_at
  }

  %% ======================= MEDIA (Mongo) =======================
  METADATA_DOCS {
    string   _id PK
    string   chainId
    string   contract
    string   tokenId
    string   tokenURI
    string   original_json
    string   normalized_json
    string   media_json
    string   normalize_status
    string   normalize_error
    datetime fetchedAt
    string   etag
  }

  MEDIA_ASSETS {
    string   _id PK
    string   cid
    string   kind
    string   source
    string   mime
    int      bytes
    int      width
    int      height
    string   s3Key
    string   ipfsCid
    string   sha256
    string   phash
    string   moderation
    string   exif_json
    datetime createdAt
  }

  MEDIA_VARIANTS {
    string   asset_id
    string   cdnUrl
    int      w
    int      h
  }

  %% ======================= RELATIONSHIPS =======================
  %% Auth/User
  USERS ||--o{ SESSIONS : sessions

  %% Wallet/User
  USERS ||--o{ WALLETS : has
  WALLETS ||--o{ APPROVALS : has
  WALLETS ||--o{ APPROVALS_HISTORY : changes

  %% Chain registry
  CHAINS ||--o{ CHAIN_ENDPOINTS : has
  CHAINS ||--o{ CHAIN_CONTRACTS : has
  CHAINS ||--|| CHAIN_GAS_POLICY : has

  %% Catalog domain
  COLLECTIONS ||--o{ COLLECTION_ROLES : roles
  COLLECTIONS ||--o{ COLLECTION_BINDINGS : has
  COLLECTIONS ||--o{ COLLECTION_MINT_CONFIG : has
  COLLECTIONS ||--o{ TOKENS : contains

  TRAITS ||--o{ TRAIT_VALUES : has
  TOKENS ||--o{ TOKEN_TRAIT_LINKS : has
  TRAITS ||--o{ TOKEN_TRAIT_LINKS : link
  TRAIT_VALUES ||--o{ TOKEN_TRAIT_LINKS : link

  MARKETPLACES ||--o{ LISTINGS : has
  MARKETPLACES ||--o{ OFFERS : has
  MARKETPLACES ||--o{ SALES : has
  TOKENS ||--o{ LISTINGS : has
  TOKENS ||--o{ OFFERS : has
  TOKENS ||--o{ SALES : has
  TOKENS ||--o{ ACTIVITIES : has

  COLLECTIONS ||--|| COLLECTION_STATS : has
  TOKENS ||--|| TOKEN_RARITY : has
  TRAIT_VALUES ||--|| TRAIT_VALUE_FLOOR : has

  TOKENS ||--o{ TOKEN_BALANCES : balances
  TOKENS ||--o{ OWNERSHIP_TRANSFERS : history
  TOKENS ||--|| NFT_FLAGS : flags

  TOKENS ||--o{ ORDERS : has
  ORDERS ||--o{ ORDER_FILLS : fills

  COLLECTIONS ||--o{ SYNC_STATE : has

  MEDIA_ASSETS ||--o{ MEDIA_VARIANTS : has
  TOKENS ||..|| METADATA_DOCS : metadata_doc

  %% Indexer/Catalog idempotency
  PROCESSED_EVENTS ||..|| LISTINGS : guard
  PROCESSED_EVENTS ||..|| OFFERS   : guard
  PROCESSED_EVENTS ||..|| SALES    : guard
  PROCESSED_EVENTS ||..|| TOKENS   : guard
  INDEXER_CHECKPOINTS ||..|| EVENTS_RAW : drive
```