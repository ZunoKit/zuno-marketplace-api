-- Migration: 000002_add_indexes.up.sql
-- Description: Add performance indexes for auth service

BEGIN;

-- ======================= PERFORMANCE INDEXES =======================

-- Composite indexes for common queries
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_user_active 
    ON sessions(user_id, expires_at) 
    WHERE revoked_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_auth_nonces_lookup 
    ON auth_nonces(account_id, chain_id, expires_at) 
    WHERE used = FALSE;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_login_events_recent 
    ON login_events(account_id, timestamp DESC) 
    WHERE timestamp > now() - INTERVAL '30 days';

-- Partial indexes for active sessions
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_active_tokens 
    ON sessions(token_family_id, token_generation) 
    WHERE revoked_at IS NULL AND expires_at > now();

-- BRIN index for time-series data
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_login_events_timestamp_brin 
    ON login_events USING BRIN(timestamp);

-- GIN index for JSONB columns
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_collection_intent_gin 
    ON sessions USING GIN(collection_intent_context) 
    WHERE collection_intent_context IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_login_events_risk_factors_gin 
    ON login_events USING GIN(risk_factors) 
    WHERE risk_factors IS NOT NULL;

-- Hash indexes for exact matches
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_refresh_hash_hash 
    ON sessions USING HASH(refresh_hash);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_device_fingerprint_hash 
    ON sessions USING HASH(device_fingerprint) 
    WHERE device_fingerprint IS NOT NULL;

-- ======================= STATISTICS =======================

-- Update statistics for better query planning
ANALYZE auth_nonces;
ANALYZE sessions;
ANALYZE login_events;

-- ======================= MONITORING VIEWS =======================

-- View for active sessions monitoring
CREATE OR REPLACE VIEW v_active_sessions AS
SELECT 
    user_id,
    COUNT(*) as session_count,
    MAX(last_used_at) as last_activity,
    COUNT(DISTINCT device_fingerprint) as unique_devices,
    COUNT(DISTINCT device_platform) as unique_platforms
FROM sessions
WHERE revoked_at IS NULL 
    AND expires_at > now()
GROUP BY user_id;

-- View for login activity monitoring
CREATE OR REPLACE VIEW v_login_activity AS
SELECT 
    DATE_TRUNC('hour', timestamp) as hour,
    COUNT(*) as total_attempts,
    COUNT(CASE WHEN result = 'success' THEN 1 END) as successful_logins,
    COUNT(CASE WHEN result != 'success' THEN 1 END) as failed_attempts,
    COUNT(DISTINCT account_id) as unique_accounts,
    AVG(risk_score) as avg_risk_score
FROM login_events
WHERE timestamp > now() - INTERVAL '24 hours'
GROUP BY DATE_TRUNC('hour', timestamp)
ORDER BY hour DESC;

-- ======================= MAINTENANCE SETTINGS =======================

-- Set autovacuum settings for high-update tables
ALTER TABLE sessions SET (
    autovacuum_vacuum_scale_factor = 0.1,
    autovacuum_analyze_scale_factor = 0.05,
    autovacuum_vacuum_cost_delay = 10
);

ALTER TABLE auth_nonces SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);

COMMIT;
