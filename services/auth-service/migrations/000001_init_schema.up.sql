-- Migration: 000001_init_schema.up.sql
-- Description: Initial schema for auth service

BEGIN;

-- Extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ======================= NONCE MANAGEMENT =======================
CREATE TABLE IF NOT EXISTS auth_nonces (
    nonce       varchar(64)  PRIMARY KEY,
    account_id  varchar(42)  NOT NULL,
    domain      varchar(255) NOT NULL,
    chain_id    varchar(32)  NOT NULL,
    issued_at   timestamptz  NOT NULL DEFAULT now(),
    expires_at  timestamptz  NOT NULL,
    used        boolean      NOT NULL DEFAULT FALSE,
    used_at     timestamptz,
    created_at  timestamptz  NOT NULL DEFAULT now()
);

-- Constraints
ALTER TABLE auth_nonces
  ADD CONSTRAINT chk_nonce_expiry
  CHECK (expires_at > issued_at AND expires_at <= issued_at + INTERVAL '10 minutes');

ALTER TABLE auth_nonces
  ADD CONSTRAINT chk_account_format
  CHECK (account_id = lower(account_id) AND account_id ~ '^0x[0-9a-f]{40}$');

-- Indexes
CREATE INDEX idx_auth_nonces_expires_at ON auth_nonces(expires_at);
CREATE INDEX idx_auth_nonces_account_id ON auth_nonces(account_id);
CREATE INDEX idx_auth_nonces_used ON auth_nonces(used);

-- ======================= SESSION MANAGEMENT =======================
CREATE TABLE IF NOT EXISTS sessions (
    session_id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               uuid NOT NULL,
    device_id             uuid,
    refresh_hash          varchar(128) NOT NULL,
    previous_refresh_hash varchar(128),
    token_family_id       uuid NOT NULL DEFAULT gen_random_uuid(),
    token_generation      integer NOT NULL DEFAULT 1,
    ip_address            inet,
    user_agent            text,
    created_at            timestamptz NOT NULL DEFAULT now(),
    expires_at            timestamptz NOT NULL,
    revoked_at            timestamptz,
    revoked_reason        varchar(255),
    last_used_at          timestamptz DEFAULT now(),
    collection_intent_context jsonb DEFAULT NULL,
    -- Device fingerprint fields
    device_fingerprint    varchar(128),
    device_platform       varchar(50),
    device_browser        varchar(100)
);

-- Indexes
CREATE UNIQUE INDEX uq_sessions_refresh_hash ON sessions(refresh_hash);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_token_family ON sessions(token_family_id);
CREATE INDEX idx_sessions_prev_refresh ON sessions(previous_refresh_hash);
CREATE INDEX idx_sessions_active_by_user ON sessions(user_id) WHERE revoked_at IS NULL;
CREATE INDEX idx_sessions_collection_context ON sessions(user_id) WHERE collection_intent_context IS NOT NULL;
CREATE INDEX idx_sessions_device_fingerprint ON sessions(device_fingerprint) WHERE device_fingerprint IS NOT NULL;

-- Constraints
ALTER TABLE sessions
  ADD CONSTRAINT chk_session_expiry CHECK (expires_at > created_at);

ALTER TABLE sessions
  ADD CONSTRAINT chk_session_revoked CHECK (revoked_at IS NULL OR revoked_at >= created_at);

-- ======================= AUDIT LOGGING =======================
CREATE TABLE IF NOT EXISTS login_events (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid,
    account_id    varchar(42) NOT NULL,
    ip_address    inet,
    user_agent    text,
    result        varchar(32) NOT NULL,
    error_message text,
    chain_id      varchar(32),
    domain        varchar(255),
    timestamp     timestamptz NOT NULL DEFAULT now(),
    -- Additional security fields
    risk_score    integer,
    risk_factors  jsonb,
    device_fingerprint varchar(128)
);

-- Constraints
ALTER TABLE login_events
  ADD CONSTRAINT chk_login_account_format
  CHECK (account_id = lower(account_id) AND account_id ~ '^0x[0-9a-f]{40}$');

ALTER TABLE login_events
  ADD CONSTRAINT chk_login_result
  CHECK (result IN ('success','failed','invalid_signature','invalid_nonce','expired_nonce','invalid_message','rate_limited','suspicious'));

-- Indexes
CREATE INDEX idx_login_events_user_id ON login_events(user_id);
CREATE INDEX idx_login_events_account_id ON login_events(account_id);
CREATE INDEX idx_login_events_timestamp ON login_events(timestamp);
CREATE INDEX idx_login_events_result ON login_events(result);
CREATE INDEX idx_login_events_ip_address ON login_events(ip_address);
CREATE INDEX idx_login_events_risk_score ON login_events(risk_score) WHERE risk_score IS NOT NULL;

-- ======================= FUNCTIONS =======================

-- Cleanup expired nonces
CREATE OR REPLACE FUNCTION cleanup_expired_nonces()
RETURNS integer AS $$
DECLARE
  deleted_count integer;
BEGIN
  DELETE FROM auth_nonces
   WHERE expires_at < now() - INTERVAL '1 hour';
  GET DIAGNOSTICS deleted_count = ROW_COUNT;
  RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Cleanup old login events
CREATE OR REPLACE FUNCTION cleanup_old_login_events(retention_days integer DEFAULT 90)
RETURNS integer AS $$
DECLARE
  deleted_count integer;
BEGIN
  DELETE FROM login_events
   WHERE timestamp < now() - (retention_days || ' days')::interval;
  GET DIAGNOSTICS deleted_count = ROW_COUNT;
  RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Cleanup expired sessions
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS integer AS $$
DECLARE
  updated_count integer;
BEGIN
  UPDATE sessions
     SET revoked_at = now(), revoked_reason = 'expired'
   WHERE expires_at < now()
     AND revoked_at IS NULL;
  GET DIAGNOSTICS updated_count = ROW_COUNT;
  RETURN updated_count;
END;
$$ LANGUAGE plpgsql;

-- CAS consume nonce
CREATE OR REPLACE FUNCTION try_use_nonce(
  p_nonce   text,
  p_account text,
  p_chain   text,
  p_domain  text
) RETURNS boolean AS $$
DECLARE
  affected integer;
BEGIN
  UPDATE auth_nonces
     SET used = TRUE, used_at = now()
   WHERE nonce = p_nonce
     AND account_id = p_account
     AND chain_id = p_chain
     AND domain = p_domain
     AND used = FALSE
     AND expires_at > now();

  GET DIAGNOSTICS affected = ROW_COUNT;
  RETURN affected = 1;
END;
$$ LANGUAGE plpgsql;

COMMIT;
