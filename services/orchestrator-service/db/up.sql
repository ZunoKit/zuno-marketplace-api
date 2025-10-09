CREATE TABLE IF NOT EXISTS tx_intents (
  intent_id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  kind             TEXT NOT NULL,                 -- 'collection' | 'mint' | ...
  chain_id         caip2_chain NOT NULL,
  preview_address  evm_address,
  tx_hash          evm_tx_hash,
  status           TEXT NOT NULL DEFAULT 'pending', -- pending|ready|failed|expired
  created_by       UUID,                          -- user_id (optional FK tới users.users nếu có)
  req_payload_json JSONB,
  error            TEXT,
  auth_session_id  VARCHAR(255),                  -- optional session correlation
  deadline_at      TIMESTAMPTZ,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_tx_intents_chain ON tx_intents(chain_id);
CREATE INDEX IF NOT EXISTS ix_tx_intents_kind ON tx_intents(kind);
CREATE INDEX IF NOT EXISTS ix_tx_intents_txhash ON tx_intents(tx_hash);

-- Add unique constraint for chain_id and tx_hash combination
-- This prevents race conditions where multiple intents could be created for the same transaction
ALTER TABLE tx_intents 
ADD CONSTRAINT unique_chain_tx 
UNIQUE (chain_id, tx_hash) 
WHERE tx_hash IS NOT NULL;
-- Session correlation index (partial)
CREATE INDEX IF NOT EXISTS idx_tx_intents_session_correlation
ON tx_intents(auth_session_id, status, created_at)
WHERE auth_session_id IS NOT NULL;

-- Audit table to record session→intent correlation
CREATE TABLE IF NOT EXISTS session_intent_audit (
  audit_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id VARCHAR(255) NOT NULL,
  intent_id UUID NOT NULL REFERENCES tx_intents(intent_id),
  user_id UUID,
  correlation_timestamp TIMESTAMPTZ NOT NULL DEFAULT now(),
  audit_data JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
