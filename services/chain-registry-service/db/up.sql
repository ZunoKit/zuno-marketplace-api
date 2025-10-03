-- =========================================================
-- Domain helpers (validate format ngay tại DB)
-- =========================================================
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'evm_address') THEN
    CREATE DOMAIN evm_address AS text
      CHECK (VALUE ~ '^0x[0-9a-f]{40}$');        -- lowercase 0x + 40 hex
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'evm_tx_hash') THEN
    CREATE DOMAIN evm_tx_hash AS text
      CHECK (VALUE ~ '^0x[0-9a-f]{64}$');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'caip2_chain') THEN
    CREATE DOMAIN caip2_chain AS text
      CHECK (VALUE ~ '^[a-z0-9]+:\d+$');         -- vd: eip155:8453
  END IF;
END$$;



-- =========================================================
-- Chains (CAIP-2 map)
-- =========================================================
CREATE TABLE IF NOT EXISTS chains (
  id               SERIAL PRIMARY KEY,
  caip2            caip2_chain UNIQUE NOT NULL,     -- e.g. eip155:1
  chain_numeric    INTEGER UNIQUE NOT NULL,         -- 1, 8453, ...
  name             TEXT NOT NULL,
  native_symbol    TEXT NOT NULL,
  decimals         INTEGER NOT NULL DEFAULT 18,
  explorer_url     TEXT,
  enabled          BOOLEAN NOT NULL DEFAULT TRUE,
  features_json    JSONB
);

-- Endpoints cho từng chain (nhiều RPC, weight/priority)
CREATE TABLE IF NOT EXISTS chain_endpoints (
  id          SERIAL PRIMARY KEY,
  chain_id    INTEGER NOT NULL REFERENCES chains(id) ON DELETE CASCADE,
  url         TEXT NOT NULL,
  priority    INTEGER NOT NULL DEFAULT 100,
  weight      INTEGER NOT NULL DEFAULT 100,
  auth_type   TEXT,
  rate_limit  INTEGER,
  active      BOOLEAN NOT NULL DEFAULT TRUE,
  UNIQUE (chain_id, url)
);
CREATE INDEX IF NOT EXISTS ix_chain_endpoints_active
  ON chain_endpoints(chain_id) WHERE active;

-- Chính sách gas mặc định theo chain (optional)
CREATE TABLE IF NOT EXISTS chain_gas_policy (
  chain_id                         INTEGER PRIMARY KEY REFERENCES chains(id) ON DELETE CASCADE,
  max_fee_gwei                     NUMERIC(20,8),
  priority_fee_gwei                NUMERIC(20,8),
  multiplier                       NUMERIC(8,4),
  last_observed_base_fee_gwei      NUMERIC(20,8),
  updated_at                       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- =========================================================
-- ABI blob metadata (payload lưu S3/IPFS theo sha256)
-- =========================================================
CREATE TABLE IF NOT EXISTS abi_blobs (
  sha256           CHAR(64) PRIMARY KEY,       -- content-addressed
  size_bytes       INTEGER NOT NULL,
  source           TEXT,                       -- etherscan|blockscout|internal
  compiler         TEXT,
  contract_name    TEXT,
  standard         TEXT,                       -- erc721|erc1155|custom|proxy|diamond
  abi_json         JSONB,                      -- store full ABI JSON
  s3_key           TEXT NOT NULL,              -- ví dụ: abis/<sha256>.json
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_abi_blobs_standard ON abi_blobs(standard);
CREATE INDEX IF NOT EXISTS ix_abi_blobs_name ON abi_blobs(contract_name);

-- =========================================================
-- Danh bạ hợp đồng theo chain + link tới ABI
-- =========================================================
CREATE TABLE IF NOT EXISTS chain_contracts (
  id             BIGSERIAL PRIMARY KEY,
  chain_id       INTEGER NOT NULL REFERENCES chains(id) ON DELETE CASCADE,
  name           TEXT,                          -- tuỳ chọn: factory/registry/collection-xyz
  address        evm_address NOT NULL,
  start_block    INTEGER,
  verified_at    TIMESTAMPTZ,
  -- Link ABI runtime:
  abi_sha256     CHAR(64) REFERENCES abi_blobs(sha256),
  impl_address   evm_address,                   -- nếu là proxy EIP-1967
  standard       TEXT,                          -- erc721|erc1155|custom|proxy|diamond
  first_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at   TIMESTAMPTZ,
  UNIQUE (chain_id, address)
);
CREATE INDEX IF NOT EXISTS ix_chain_contracts_chain ON chain_contracts(chain_id);
CREATE INDEX IF NOT EXISTS ix_chain_contracts_abi ON chain_contracts(abi_sha256);
CREATE INDEX IF NOT EXISTS ix_chain_contracts_impl ON chain_contracts(impl_address);
CREATE INDEX IF NOT EXISTS ix_chain_contracts_standard ON chain_contracts(standard);

-- Lịch sử nâng cấp proxy (để reprocess/đối chiếu)
CREATE TABLE IF NOT EXISTS contract_impl_history (
  id                  BIGSERIAL PRIMARY KEY,
  chain_contract_id   BIGINT NOT NULL REFERENCES chain_contracts(id) ON DELETE CASCADE,
  prev_impl           evm_address,
  new_impl            evm_address NOT NULL,
  tx_hash             evm_tx_hash,
  block_number        INTEGER,
  at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_contract_impl_hist_cc ON contract_impl_history(chain_contract_id, block_number);

-- Diamond facets (tuỳ chọn, nếu dùng EIP-2535)
CREATE TABLE IF NOT EXISTS diamond_facets (
  id                  BIGSERIAL PRIMARY KEY,
  chain_contract_id   BIGINT NOT NULL REFERENCES chain_contracts(id) ON DELETE CASCADE,
  facet_address       evm_address NOT NULL,
  abi_sha256          CHAR(64) REFERENCES abi_blobs(sha256),
  selector_count      INTEGER,
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(chain_contract_id, facet_address)
);

-- Map 4byte selector → tên hàm tại thời điểm ghi nhận (optional, hỗ trợ decode ví/FE)
CREATE TABLE IF NOT EXISTS contract_selectors (
  id                  BIGSERIAL PRIMARY KEY,
  chain_contract_id   BIGINT NOT NULL REFERENCES chain_contracts(id) ON DELETE CASCADE,
  selector            BYTEA NOT NULL,     -- 4 bytes
  name                TEXT,               -- ví dụ "mint(address,uint256)"
  UNIQUE(chain_contract_id, selector)
);

-- =========================================================
-- Seed initial chains and RPC endpoints
-- =========================================================

-- Chains: Anvil (local) and Sepolia
INSERT INTO chains (caip2, chain_numeric, name, native_symbol, decimals, explorer_url, enabled, features_json)
VALUES
  ('eip155:31337', 31337, 'Anvil (Local)', 'ETH', 18, 'https://anvil.etherscan.io', TRUE, '{}'::jsonb),
  ('eip155:11155111', 11155111, 'Sepolia', 'ETH', 18, 'https://sepolia.etherscan.io', TRUE, '{}'::jsonb)
ON CONFLICT DO NOTHING;

-- RPC endpoints for Anvil (local)
INSERT INTO chain_endpoints (chain_id, url, priority, weight, auth_type, rate_limit, active)
SELECT id, 'http://anvil:8545', 10, 100, 'NONE', NULL, FALSE
FROM chains WHERE caip2 = 'eip155:31337'
ON CONFLICT DO NOTHING;


-- RPC endpoints for Sepolia (public)
INSERT INTO chain_endpoints (chain_id, url, priority, weight, auth_type, rate_limit, active)
SELECT id, 'https://rpc.sepolia.org', 10, 100, 'NONE', NULL, TRUE
FROM chains WHERE caip2 = 'eip155:11155111'
ON CONFLICT DO NOTHING;

-- Default gas policies
INSERT INTO chain_gas_policy (chain_id, max_fee_gwei, priority_fee_gwei, multiplier, last_observed_base_fee_gwei)
SELECT id, 50.0, 2.0, 1.10, 20.0 FROM chains WHERE caip2 = 'eip155:31337'
ON CONFLICT (chain_id) DO NOTHING;

INSERT INTO chain_gas_policy (chain_id, max_fee_gwei, priority_fee_gwei, multiplier, last_observed_base_fee_gwei)
SELECT id, 50.0, 2.0, 1.10, 20.0 FROM chains WHERE caip2 = 'eip155:11155111'
ON CONFLICT (chain_id) DO NOTHING;

