

-- Extensions (for gen_random_uuid)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------- USERS ----------
CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status     VARCHAR(50) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT users_status_check CHECK (status IN ('active', 'suspended', 'deleted'))
);

CREATE INDEX IF NOT EXISTS idx_users_status     ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

-- ---------- PROFILES ----------
CREATE TABLE IF NOT EXISTS profiles (
    user_id      UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    username     VARCHAR(50),
    display_name VARCHAR(100),
    avatar_url   TEXT,
    banner_url   TEXT,
    bio          TEXT CHECK (length(bio) <= 500),
    locale       VARCHAR(10)  NOT NULL DEFAULT 'en',
    timezone     VARCHAR(50)  NOT NULL DEFAULT 'UTC',
    socials_json JSONB        NOT NULL DEFAULT '{}',
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT profiles_username_length  CHECK (username IS NULL OR (length(username) BETWEEN 3 AND 50)),
    CONSTRAINT profiles_username_format  CHECK (username IS NULL OR username ~ '^[a-zA-Z0-9_-]+$'),
    CONSTRAINT profiles_display_name_len CHECK (display_name IS NULL OR length(display_name) <= 100),
    CONSTRAINT profiles_avatar_url_fmt   CHECK (avatar_url IS NULL OR avatar_url ~ '^https?://'),
    CONSTRAINT profiles_banner_url_fmt   CHECK (banner_url IS NULL OR banner_url ~ '^https?://')
);

CREATE INDEX IF NOT EXISTS idx_profiles_username    ON profiles(username) WHERE username IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_profiles_updated_at  ON profiles(updated_at);

-- Unique (case-insensitive) on username
CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_username_unique
    ON profiles (LOWER(username))
    WHERE username IS NOT NULL;

-- Auto-update updated_at on profiles
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS update_profiles_updated_at ON profiles;
CREATE TRIGGER update_profiles_updated_at
BEFORE UPDATE ON profiles
FOR EACH ROW
EXECUTE FUNCTION update_updated_at_column();

-- ---------- USER_ACCOUNTS ----------
-- Mapping: 1 account_id -> 1 user_id (PK = account_id)
CREATE TABLE IF NOT EXISTS user_accounts (
    account_id   VARCHAR(255) PRIMARY KEY,                -- external account identifier
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    address      VARCHAR(42) NOT NULL,                    -- lowercase EVM address
    chain_id     TEXT,                                    -- CAIP-2 (optional)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT user_accounts_address_format CHECK (address ~ '^0x[a-f0-9]{40}$')
);

-- Helpful indexes
CREATE INDEX IF NOT EXISTS idx_user_accounts_user_id       ON user_accounts(user_id);
CREATE INDEX IF NOT EXISTS idx_user_accounts_address       ON user_accounts(address);
CREATE INDEX IF NOT EXISTS idx_user_accounts_created_at    ON user_accounts(created_at);
CREATE INDEX IF NOT EXISTS idx_user_accounts_last_seen_at  ON user_accounts(last_seen_at);
