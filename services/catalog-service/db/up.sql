-- catalog_db schema
-- Recommended extensions (optional)
-- CREATE EXTENSION IF NOT EXISTS "pgcrypto"; -- for gen_random_uuid()

-- =========================
-- Collections & bindings
-- =========================
CREATE TABLE IF NOT EXISTS collections (
  id                   uuid PRIMARY KEY,
  slug                 text UNIQUE,
  name                 text NOT NULL,
  description          text,
  chain_id             text NOT NULL,
  contract_address     text NOT NULL,
  creator              text NOT NULL,
  tx_hash              text,
  owner                text,
  collection_type      text NOT NULL,
  max_supply           text DEFAULT '0',
  total_supply         text DEFAULT '0',
  royalty_recipient    text,
  royalty_percentage   integer DEFAULT 0 CHECK (royalty_percentage BETWEEN 0 AND 10000),
  
  -- Minting Configuration Fields from CollectionParams
  mint_price              text DEFAULT '0',
  royalty_fee             text DEFAULT '0',
  mint_limit_per_wallet   text DEFAULT '0', 
  mint_start_time         text DEFAULT '0',
  allowlist_mint_price    text DEFAULT '0',
  public_mint_price       text DEFAULT '0',
  allowlist_stage_duration text DEFAULT '0',
  token_uri               text,
  
  is_verified          boolean DEFAULT false,
  is_explicit          boolean DEFAULT false,
  is_featured          boolean DEFAULT false,
  image_url            text,
  banner_url           text,
  external_url         text,
  discord_url          text,
  twitter_url          text,
  instagram_url        text,
  telegram_url         text,
  floor_price          text DEFAULT '0',
  volume_traded        text DEFAULT '0',
  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now(),
  UNIQUE(chain_id, contract_address)
);
CREATE INDEX IF NOT EXISTS idx_collections_chain_contract ON collections(chain_id, contract_address);
CREATE INDEX IF NOT EXISTS idx_collections_creator ON collections(creator);
CREATE INDEX IF NOT EXISTS idx_collections_tx_hash ON collections(tx_hash);

CREATE TABLE IF NOT EXISTS collection_roles (
  chain_id     text NOT NULL,
  address      text NOT NULL,
  role         text NOT NULL,
  account      text NOT NULL,
  granted_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (chain_id, address, role, account)
);

CREATE TABLE IF NOT EXISTS collection_bindings (
  id                uuid PRIMARY KEY,
  collection_id     uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  chain_id          text NOT NULL,
  family            text,           -- e.g. evm/solana/bitcoin
  token_standard    text,           -- e.g. ERC721/1155/inscriptions
  contract_address  text,
  mint_authority    text,
  inscription_id    text,
  is_primary        boolean DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_collection_bind_primary
  ON collection_bindings(collection_id)
  WHERE is_primary = true;
CREATE INDEX IF NOT EXISTS idx_collection_bind_contract
  ON collection_bindings(chain_id, contract_address);

CREATE TABLE IF NOT EXISTS collection_mint_config (
  collection_id   uuid PRIMARY KEY REFERENCES collections(id) ON DELETE CASCADE,
  start_date      timestamptz,
  end_date        timestamptz,
  mint_price_text text
);

-- =========================
-- Tokens & traits
-- =========================
CREATE TABLE IF NOT EXISTS tokens (
  id                uuid PRIMARY KEY,
  collection_id     uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  chain_id          text NOT NULL,
  family            text,
  contract_address  text,
  mint_address      text,
  inscription_id    text,
  token_number      text,        -- string to handle big ints / non-numeric
  token_standard    text,
  supply            integer,
  burned            boolean DEFAULT false,
  name              text,
  image_url         text,
  metadata_url      text,
  owner_address     text,
  minted_block      integer,
  minted_at         timestamptz,
  last_refresh_at   timestamptz,
  metadata_doc      text
);
-- Fast lookup by collection + token_number
CREATE UNIQUE INDEX IF NOT EXISTS uq_tokens_collection_token_number
  ON tokens(collection_id, token_number);
CREATE INDEX IF NOT EXISTS idx_tokens_contract_token
  ON tokens(chain_id, contract_address, token_number);

CREATE TABLE IF NOT EXISTS traits (
  id               uuid PRIMARY KEY,
  collection_id    uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  name             text NOT NULL,
  normalized_name  text NOT NULL,
  value_type       text,          -- string/number/date/bool/…
  display_type     text,
  unit             text,
  sort_order       integer DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_traits_collection_name
  ON traits(collection_id, normalized_name);

CREATE TABLE IF NOT EXISTS trait_values (
  id                  uuid PRIMARY KEY,
  trait_id            uuid NOT NULL REFERENCES traits(id) ON DELETE CASCADE,
  value_type          text,
  value_string        text,
  value_number        double precision,
  value_epoch_seconds bigint,
  normalized_value    text,
  occurrences         integer,
  frequency           double precision,
  rarity_score        double precision,
  max_value           double precision,
  unit                text
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_trait_values_unique
  ON trait_values(trait_id, coalesce(normalized_value,''), coalesce(value_string,''), coalesce(value_number, -1e309));

CREATE TABLE IF NOT EXISTS token_trait_links (
  token_id        uuid NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
  trait_id        uuid NOT NULL REFERENCES traits(id) ON DELETE CASCADE,
  trait_value_id  uuid NOT NULL REFERENCES trait_values(id) ON DELETE CASCADE,
  PRIMARY KEY (token_id, trait_id, trait_value_id)
);

-- =========================
-- Balances, ownership & flags
-- (the design keeps chain/contract/token_id as text keys)
-- =========================
CREATE TABLE IF NOT EXISTS token_balances (
  chain_id   text NOT NULL,
  contract   text NOT NULL,
  token_id   text NOT NULL,
  owner      text NOT NULL,
  quantity   numeric NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (chain_id, contract, token_id, owner)
);
CREATE INDEX IF NOT EXISTS idx_balances_owner ON token_balances(owner);

CREATE TABLE IF NOT EXISTS ownership_transfers (
  chain_id   text NOT NULL,
  contract   text NOT NULL,
  token_id   text NOT NULL,
  from_addr  text NOT NULL,
  to_addr    text NOT NULL,
  tx_hash    text NOT NULL,
  log_index  integer NOT NULL,
  at         timestamptz NOT NULL,
  PRIMARY KEY (chain_id, contract, token_id, tx_hash, log_index)
);
CREATE INDEX IF NOT EXISTS idx_transfers_to ON ownership_transfers(to_addr);

CREATE TABLE IF NOT EXISTS nft_flags (
  chain_id     text NOT NULL,
  contract     text NOT NULL,
  token_id     text NOT NULL,
  is_flagged   boolean DEFAULT false,
  is_spam      boolean DEFAULT false,
  is_frozen    boolean DEFAULT false,
  is_nsfw      boolean DEFAULT false,
  refreshable  boolean DEFAULT true,
  reason_json  jsonb DEFAULT '{}'::jsonb,
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (chain_id, contract, token_id)
);

-- =========================
-- Market data: marketplaces, listings, offers, sales
-- =========================
CREATE TABLE IF NOT EXISTS marketplaces (
  id   uuid PRIMARY KEY,
  name text NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS listings (
  id                 uuid PRIMARY KEY,
  token_id           uuid NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
  marketplace_id     uuid NOT NULL REFERENCES marketplaces(id),
  price_native       numeric,
  price_native_text  text,
  currency_symbol    text,
  is_active          boolean DEFAULT true,
  listed_at          timestamptz,
  updated_at         timestamptz,
  seller_address     text,
  expires_at         timestamptz,
  url                text,
  tx_hash            text
);
CREATE INDEX IF NOT EXISTS idx_listings_token_active ON listings(token_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_listings_market_active ON listings(marketplace_id) WHERE is_active = true;

CREATE TABLE IF NOT EXISTS offers (
  id                 uuid PRIMARY KEY,
  token_id           uuid NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
  marketplace_id     uuid NOT NULL REFERENCES marketplaces(id),
  price_native       numeric,
  price_native_text  text,
  currency_symbol    text,
  from_address       text,
  created_at         timestamptz,
  expires_at         timestamptz,
  tx_hash            text
);
CREATE INDEX IF NOT EXISTS idx_offers_token ON offers(token_id);
CREATE INDEX IF NOT EXISTS idx_offers_created ON offers(created_at);

CREATE TABLE IF NOT EXISTS sales (
  id                 uuid PRIMARY KEY,
  token_id           uuid NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
  marketplace_id     uuid NOT NULL REFERENCES marketplaces(id),
  price_native       numeric,
  price_native_text  text,
  currency_symbol    text,
  tx_hash            text NOT NULL,
  occurred_at        timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sales_token_time ON sales(token_id, occurred_at DESC);

-- =========================
-- Orders (optional generalization) & fills
-- =========================
CREATE TABLE IF NOT EXISTS orders (
  id                 uuid PRIMARY KEY,
  token_id           uuid NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
  side               text,           -- buy/sell
  maker              text,
  taker              text,
  price_native       numeric,
  currency_symbol    text,
  start_at           timestamptz,
  end_at             timestamptz,
  signature          text,
  salt               text,
  source_marketplace text,
  status             text,           -- open/filled/cancelled/expired
  updated_at         timestamptz
);
CREATE INDEX IF NOT EXISTS idx_orders_token_status ON orders(token_id, status);

CREATE TABLE IF NOT EXISTS order_fills (
  id           uuid PRIMARY KEY,
  order_id     uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  tx_hash      text NOT NULL,
  price_native numeric,
  filled_at    timestamptz NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_order_fills_tx ON order_fills(tx_hash, order_id);

-- =========================
-- Stats, rarity & trait floors
-- =========================
CREATE TABLE IF NOT EXISTS collection_stats (
  collection_id        uuid PRIMARY KEY REFERENCES collections(id) ON DELETE CASCADE,
  items_count          integer,
  owners_count         integer,
  floor_price_native   numeric,
  floor_currency_symbol text,
  market_cap_est       numeric,
  last_updated_at      timestamptz
);

CREATE TABLE IF NOT EXISTS token_rarity (
  token_id               uuid PRIMARY KEY REFERENCES tokens(id) ON DELETE CASCADE,
  rarity_score_product   double precision
);

CREATE TABLE IF NOT EXISTS rarity_scores (
  token_id   uuid NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
  method     text NOT NULL,
  source     text,
  score      double precision,
  rank       integer,
  updated_at timestamptz,
  PRIMARY KEY (token_id, method)
);

CREATE TABLE IF NOT EXISTS trait_value_floor (
  trait_value_id       uuid PRIMARY KEY REFERENCES trait_values(id) ON DELETE CASCADE,
  floor_price_native   numeric,
  last_updated_at      timestamptz
);

-- =========================
-- Activities feed
-- =========================
CREATE TABLE IF NOT EXISTS activities (
  id                 uuid PRIMARY KEY,
  token_id           uuid NOT NULL REFERENCES tokens(id) ON DELETE CASCADE,
  type               text NOT NULL,      -- listed, offer, sale, transfer, mint, burn, …
  from_address       text,
  to_address         text,
  price_native       numeric,
  price_native_text  text,
  currency_symbol    text,
  tx_hash            text,
  block_or_slot      text,
  timestamp          timestamptz NOT NULL,
  marketplace        text
);
CREATE INDEX IF NOT EXISTS idx_activities_token_time ON activities(token_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_activities_type_time  ON activities(type, timestamp DESC);

-- =========================
-- Sync state for external sources
-- =========================
CREATE TABLE IF NOT EXISTS sync_state (
  id             uuid PRIMARY KEY,
  collection_id  uuid NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
  source         text NOT NULL,         -- e.g. opensea, magiceden, custom-indexer
  cursor         text,
  last_run_at    timestamptz,
  note           text
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_sync_state_source
  ON sync_state(collection_id, source);

-- =========================
-- Idempotency guard for domain upserts
-- =========================
CREATE TABLE IF NOT EXISTS processed_events (
  event_id      text PRIMARY KEY,
  event_version integer NOT NULL DEFAULT 1,
  chain_id      text,
  block_hash    text,
  log_index     integer,
  processed_at  timestamptz NOT NULL DEFAULT now()
);
-- Helpful for dedupe/inspection
CREATE INDEX IF NOT EXISTS idx_processed_chain_block ON processed_events(chain_id, block_hash);

-- =========================
-- Triggers (optional): touch updated_at on collections
-- =========================
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'collections_touch_updated_at') THEN
    CREATE OR REPLACE FUNCTION collections_touch_updated_at()
    RETURNS trigger AS $f$
    BEGIN
      NEW.updated_at := now();
      RETURN NEW;
    END;$f$ LANGUAGE plpgsql;
  END IF;
END$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_trigger WHERE tgname = 'trg_collections_touch_updated_at'
  ) THEN
    CREATE TRIGGER trg_collections_touch_updated_at
    BEFORE UPDATE ON collections
    FOR EACH ROW EXECUTE FUNCTION collections_touch_updated_at();
  END IF;
END$$;
