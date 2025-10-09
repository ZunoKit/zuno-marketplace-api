-- Migration: 000002_add_indexes.down.sql
-- Description: Remove performance indexes

BEGIN;

-- Drop views
DROP VIEW IF EXISTS v_login_activity;
DROP VIEW IF EXISTS v_active_sessions;

-- Drop indexes
DROP INDEX IF EXISTS idx_sessions_device_fingerprint_hash;
DROP INDEX IF EXISTS idx_sessions_refresh_hash_hash;
DROP INDEX IF EXISTS idx_login_events_risk_factors_gin;
DROP INDEX IF EXISTS idx_sessions_collection_intent_gin;
DROP INDEX IF EXISTS idx_login_events_timestamp_brin;
DROP INDEX IF EXISTS idx_sessions_active_tokens;
DROP INDEX IF EXISTS idx_login_events_recent;
DROP INDEX IF EXISTS idx_auth_nonces_lookup;
DROP INDEX IF EXISTS idx_sessions_user_active;

COMMIT;
