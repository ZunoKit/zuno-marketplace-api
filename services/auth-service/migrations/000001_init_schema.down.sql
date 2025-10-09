-- Migration: 000001_init_schema.down.sql
-- Description: Rollback initial schema for auth service

BEGIN;

-- Drop functions
DROP FUNCTION IF EXISTS try_use_nonce(text, text, text, text);
DROP FUNCTION IF EXISTS cleanup_expired_sessions();
DROP FUNCTION IF EXISTS cleanup_old_login_events(integer);
DROP FUNCTION IF EXISTS cleanup_expired_nonces();

-- Drop tables
DROP TABLE IF EXISTS login_events;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS auth_nonces;

COMMIT;
