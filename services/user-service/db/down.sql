-- User Service Database Schema Rollback (down.sql)
-- Drops objects created by the corresponding up.sql

-- 1) Drop triggers first (depends on function & table)
DROP TRIGGER IF EXISTS update_profiles_updated_at ON profiles;

-- 2) Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- 3) Drop indexes (safe even if tables will be dropped next)
-- Profiles
DROP INDEX IF EXISTS idx_profiles_username_unique;
DROP INDEX IF EXISTS idx_profiles_updated_at;
DROP INDEX IF EXISTS idx_profiles_username;

-- Users
DROP INDEX IF EXISTS idx_users_created_at;
DROP INDEX IF EXISTS idx_users_status;

-- User Accounts
DROP INDEX IF EXISTS idx_user_accounts_last_seen_at;
DROP INDEX IF EXISTS idx_user_accounts_created_at;
DROP INDEX IF EXISTS idx_user_accounts_address;
DROP INDEX IF EXISTS idx_user_accounts_user_id;

-- 4) Drop tables in reverse dependency order
DROP TABLE IF EXISTS user_accounts;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS users;

-- 5) Optional: only drop if you created it just for this schema and it's unused elsewhere
-- DROP EXTENSION IF EXISTS pgcrypto;
