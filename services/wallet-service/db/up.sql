BEGIN;

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ======================= WALLET LINKS =======================
CREATE TABLE IF NOT EXISTS wallet_links (
    wallet_id    uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid         NOT NULL,  -- References users table from user service
    account_id   varchar(255) NOT NULL,  -- Account identifier (e.g., EOA address)
    address      varchar(42)  NOT NULL,  -- Ethereum address (lowercase 0x...)
    chain_id     varchar(32)  NOT NULL,  -- CAIP-2 format (e.g., "eip155:1")
    is_primary   boolean      NOT NULL DEFAULT false,
    type         varchar(20)  DEFAULT 'eoa',  -- eoa, contract, etc.
    connector    varchar(50),              -- metamask, walletconnect, etc.
    label        varchar(100),             -- User-defined label
    verified_at  timestamptz  NOT NULL DEFAULT now(),
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);

-- Fallback for UUID generation if pgcrypto is not available
DO $$
BEGIN
  PERFORM gen_random_uuid();
EXCEPTION WHEN undefined_function THEN
  EXECUTE 'ALTER TABLE wallet_links ALTER COLUMN wallet_id SET DEFAULT uuid_generate_v4()';
END;
$$;

-- Address validation (lowercase 0x + 40 hex characters)
ALTER TABLE wallet_links
  DROP CONSTRAINT IF EXISTS chk_address_format,
  ADD  CONSTRAINT chk_address_format
  CHECK (address = lower(address) AND address ~ '^0x[0-9a-f]{40}$');

-- Chain ID validation (CAIP-2 format)
ALTER TABLE wallet_links
  DROP CONSTRAINT IF EXISTS chk_chain_id_format,
  ADD  CONSTRAINT chk_chain_id_format
  CHECK (chain_id ~ '^[a-z0-9]+:[a-zA-Z0-9]+$');

-- Type validation
ALTER TABLE wallet_links
  DROP CONSTRAINT IF EXISTS chk_wallet_type,
  ADD  CONSTRAINT chk_wallet_type
  CHECK (type IN ('eoa', 'contract', 'multisig', 'smart_account'));

-- Unique constraint: one address per user per chain
CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_user_address_chain 
  ON wallet_links(user_id, address, chain_id);

-- Only one primary wallet per user
CREATE UNIQUE INDEX IF NOT EXISTS uq_wallet_primary 
  ON wallet_links(user_id) 
  WHERE is_primary = true;

-- Indexes for queries
CREATE INDEX IF NOT EXISTS idx_wallet_links_user_id ON wallet_links(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_links_address ON wallet_links(address);
CREATE INDEX IF NOT EXISTS idx_wallet_links_chain_id ON wallet_links(chain_id);
CREATE INDEX IF NOT EXISTS idx_wallet_links_created_at ON wallet_links(created_at);

-- ======================= WALLET ACTIVITY =======================
CREATE TABLE IF NOT EXISTS wallet_activity (
    id           uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id    uuid         NOT NULL REFERENCES wallet_links(wallet_id) ON DELETE CASCADE,
    user_id      uuid         NOT NULL,
    action       varchar(50)  NOT NULL,  -- linked, unlinked, set_primary, verified
    metadata     jsonb,                  -- Additional activity metadata
    ip_address   inet,
    user_agent   text,
    created_at   timestamptz  NOT NULL DEFAULT now()
);

-- Fallback for UUID generation
DO $$
BEGIN
  PERFORM gen_random_uuid();
EXCEPTION WHEN undefined_function THEN
  EXECUTE 'ALTER TABLE wallet_activity ALTER COLUMN id SET DEFAULT uuid_generate_v4()';
END;
$$;

-- Action validation
ALTER TABLE wallet_activity
  DROP CONSTRAINT IF EXISTS chk_activity_action,
  ADD  CONSTRAINT chk_activity_action
  CHECK (action IN ('linked', 'unlinked', 'set_primary', 'verified', 'updated'));

-- Indexes for activity queries
CREATE INDEX IF NOT EXISTS idx_wallet_activity_wallet_id ON wallet_activity(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_activity_user_id ON wallet_activity(user_id);
CREATE INDEX IF NOT EXISTS idx_wallet_activity_created_at ON wallet_activity(created_at);
CREATE INDEX IF NOT EXISTS idx_wallet_activity_action ON wallet_activity(action);

-- ======================= WALLET VERIFICATION =======================
CREATE TABLE IF NOT EXISTS wallet_verifications (
    id               uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id        uuid         NOT NULL REFERENCES wallet_links(wallet_id) ON DELETE CASCADE,
    verification_type varchar(50) NOT NULL,  -- signature, transaction, etc.
    verification_data jsonb      NOT NULL,
    status           varchar(20)  NOT NULL DEFAULT 'pending',
    verified_at      timestamptz,
    expires_at       timestamptz,
    created_at       timestamptz  NOT NULL DEFAULT now()
);

-- Fallback for UUID generation
DO $$
BEGIN
  PERFORM gen_random_uuid();
EXCEPTION WHEN undefined_function THEN
  EXECUTE 'ALTER TABLE wallet_verifications ALTER COLUMN id SET DEFAULT uuid_generate_v4()';
END;
$$;

-- Status validation
ALTER TABLE wallet_verifications
  DROP CONSTRAINT IF EXISTS chk_verification_status,
  ADD  CONSTRAINT chk_verification_status
  CHECK (status IN ('pending', 'verified', 'failed', 'expired'));

-- Indexes
CREATE INDEX IF NOT EXISTS idx_wallet_verifications_wallet_id ON wallet_verifications(wallet_id);
CREATE INDEX IF NOT EXISTS idx_wallet_verifications_status ON wallet_verifications(status);
CREATE INDEX IF NOT EXISTS idx_wallet_verifications_expires_at ON wallet_verifications(expires_at);

-- ======================= FUNCTIONS =======================

-- Function to ensure only one primary wallet per user
CREATE OR REPLACE FUNCTION ensure_single_primary_wallet()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_primary = true THEN
        -- Set all other wallets for this user to non-primary
        UPDATE wallet_links
        SET is_primary = false
        WHERE user_id = NEW.user_id
          AND wallet_id != NEW.wallet_id
          AND is_primary = true;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to ensure single primary wallet
CREATE TRIGGER ensure_single_primary
    BEFORE INSERT OR UPDATE OF is_primary ON wallet_links
    FOR EACH ROW
    WHEN (NEW.is_primary = true)
    EXECUTE FUNCTION ensure_single_primary_wallet();

-- Function to log wallet activity
CREATE OR REPLACE FUNCTION log_wallet_activity()
RETURNS TRIGGER AS $$
DECLARE
    v_action varchar(50);
    v_metadata jsonb;
BEGIN
    IF TG_OP = 'INSERT' THEN
        v_action := 'linked';
        v_metadata := jsonb_build_object(
            'address', NEW.address,
            'chain_id', NEW.chain_id,
            'is_primary', NEW.is_primary
        );
    ELSIF TG_OP = 'UPDATE' THEN
        IF OLD.is_primary != NEW.is_primary AND NEW.is_primary = true THEN
            v_action := 'set_primary';
        ELSE
            v_action := 'updated';
        END IF;
        v_metadata := jsonb_build_object(
            'old', jsonb_build_object('is_primary', OLD.is_primary, 'label', OLD.label),
            'new', jsonb_build_object('is_primary', NEW.is_primary, 'label', NEW.label)
        );
    ELSIF TG_OP = 'DELETE' THEN
        v_action := 'unlinked';
        v_metadata := jsonb_build_object(
            'address', OLD.address,
            'chain_id', OLD.chain_id
        );
    END IF;

    INSERT INTO wallet_activity (wallet_id, user_id, action, metadata)
    VALUES (
        COALESCE(NEW.wallet_id, OLD.wallet_id),
        COALESCE(NEW.user_id, OLD.user_id),
        v_action,
        v_metadata
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for wallet activity logging
CREATE TRIGGER log_wallet_changes
    AFTER INSERT OR UPDATE OR DELETE ON wallet_links
    FOR EACH ROW
    EXECUTE FUNCTION log_wallet_activity();

-- ======================= COMMENTS =======================
COMMENT ON TABLE wallet_links IS 'User wallet connections and metadata';
COMMENT ON COLUMN wallet_links.account_id IS 'Account identifier (e.g., EOA address)';
COMMENT ON COLUMN wallet_links.address IS 'Ethereum address in lowercase format';
COMMENT ON COLUMN wallet_links.chain_id IS 'CAIP-2 chain identifier';
COMMENT ON COLUMN wallet_links.type IS 'Wallet type: eoa, contract, multisig, smart_account';

COMMENT ON TABLE wallet_activity IS 'Audit log of wallet-related actions';
COMMENT ON TABLE wallet_verifications IS 'Wallet ownership verification records';

COMMIT;
