BEGIN;

-- Extensions (dùng cả hai để tương thích môi trường)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ======================= NONCE MANAGEMENT =======================
-- Dùng timestamptz để tránh lệch timezone, thêm used_at để audit
CREATE TABLE IF NOT EXISTS auth_nonces (
    nonce       varchar(64)  PRIMARY KEY,             -- hex string duy nhất
    account_id  varchar(42)  NOT NULL,                -- ví EVM (0x...)
    domain      varchar(255) NOT NULL,                -- domain yêu cầu xác thực
    chain_id    varchar(32)  NOT NULL,                -- CAIP-2 (vd: eip155:1)
    issued_at   timestamptz  NOT NULL DEFAULT now(),
    expires_at  timestamptz  NOT NULL,                -- TTL mặc định 5'
    used        boolean      NOT NULL DEFAULT FALSE,  -- đã dùng chưa
    used_at     timestamptz,                          -- thời điểm consume
    created_at  timestamptz  NOT NULL DEFAULT now()
);

-- Ràng buộc thời hạn nonce: > issued_at và <= 10 phút
ALTER TABLE auth_nonces
  DROP CONSTRAINT IF EXISTS chk_nonce_expiry,
  ADD  CONSTRAINT chk_nonce_expiry
  CHECK (expires_at > issued_at AND expires_at <= issued_at + INTERVAL '10 minutes');

-- Chuẩn hoá & validate địa chỉ ví: bắt buộc lowercase 0x + hex 40 ký tự
ALTER TABLE auth_nonces
  DROP CONSTRAINT IF EXISTS chk_account_format,
  ADD  CONSTRAINT chk_account_format
  CHECK (account_id = lower(account_id) AND account_id ~ '^0x[0-9a-f]{40}$');

-- Index phục vụ tra cứu/cleanup
CREATE INDEX IF NOT EXISTS idx_auth_nonces_expires_at  ON auth_nonces(expires_at);
CREATE INDEX IF NOT EXISTS idx_auth_nonces_account_id  ON auth_nonces(account_id);
CREATE INDEX IF NOT EXISTS idx_auth_nonces_used        ON auth_nonces(used);

-- ======================= SESSION MANAGEMENT =======================
CREATE TABLE IF NOT EXISTS sessions (
    session_id   uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid         NOT NULL,                  -- tham chiếu user service
    device_id    uuid,                                   -- tuỳ chọn theo dõi thiết bị
    refresh_hash varchar(128) NOT NULL,                  -- HASH của refresh token
    ip_address   inet,                                   -- IP client
    user_agent   text,                                   -- UA client
    created_at   timestamptz  NOT NULL DEFAULT now(),
    expires_at   timestamptz  NOT NULL,                  -- hết hạn session
    revoked_at   timestamptz,                            -- khi bị revoke
    last_used_at timestamptz  DEFAULT now()              -- hoạt động gần nhất
);

-- Nếu gen_random_uuid() không tồn tại (pgcrypto chưa bật), fallback sang uuid-ossp
DO $$
BEGIN
  PERFORM gen_random_uuid();
EXCEPTION WHEN undefined_function THEN
  EXECUTE 'ALTER TABLE sessions ALTER COLUMN session_id SET DEFAULT uuid_generate_v4()';
END;
$$;

-- Index & unique cho quản lý session
CREATE UNIQUE INDEX IF NOT EXISTS uq_sessions_refresh_hash ON sessions(refresh_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id           ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at        ON sessions(expires_at);

-- Partial index: truy vấn session active theo user nhanh
CREATE INDEX IF NOT EXISTS idx_sessions_active_by_user
  ON sessions(user_id)
  WHERE revoked_at IS NULL;

-- Ràng buộc tính hợp lệ của thời gian
ALTER TABLE sessions
  DROP CONSTRAINT IF EXISTS chk_session_expiry,
  ADD  CONSTRAINT chk_session_expiry CHECK (expires_at > created_at);

ALTER TABLE sessions
  DROP CONSTRAINT IF EXISTS chk_session_revoked,
  ADD  CONSTRAINT chk_session_revoked CHECK (revoked_at IS NULL OR revoked_at >= created_at);

-- ======================= ENHANCED SESSION CONTEXT =======================
ALTER TABLE sessions
  ADD COLUMN IF NOT EXISTS collection_intent_context JSONB DEFAULT NULL;

-- Partial index: truy vấn session có collection context theo user
CREATE INDEX IF NOT EXISTS idx_sessions_collection_context
  ON sessions(user_id)
  WHERE collection_intent_context IS NOT NULL;

-- ======================= AUDIT LOGGING =======================
CREATE TABLE IF NOT EXISTS login_events (
    id           uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid,                                 -- có thể NULL nếu fail trước khi map user
    account_id   varchar(42)  NOT NULL,                -- ví EVM dùng đăng nhập
    ip_address   inet,
    user_agent   text,
    result       varchar(32)  NOT NULL,                -- 'success','failed','invalid_signature',...
    error_message text,
    chain_id     varchar(32),                          -- CAIP-2
    domain       varchar(255),
    timestamp    timestamptz  NOT NULL DEFAULT now()
);

-- Fallback default UUID cho login_events nếu thiếu pgcrypto
DO $$
BEGIN
  PERFORM gen_random_uuid();
EXCEPTION WHEN undefined_function THEN
  EXECUTE 'ALTER TABLE login_events ALTER COLUMN id SET DEFAULT uuid_generate_v4()';
END;
$$;

-- Validate địa chỉ ví lowercase
ALTER TABLE login_events
  DROP CONSTRAINT IF EXISTS chk_login_account_format,
  ADD  CONSTRAINT chk_login_account_format
  CHECK (account_id = lower(account_id) AND account_id ~ '^0x[0-9a-f]{40}$');

-- Validate tập kết quả
ALTER TABLE login_events
  DROP CONSTRAINT IF EXISTS chk_login_result,
  ADD  CONSTRAINT chk_login_result
  CHECK (result IN ('success','failed','invalid_signature','invalid_nonce','expired_nonce','invalid_message','rate_limited'));

-- Index cho audit/truy vấn
CREATE INDEX IF NOT EXISTS idx_login_events_user_id     ON login_events(user_id);
CREATE INDEX IF NOT EXISTS idx_login_events_account_id  ON login_events(account_id);
CREATE INDEX IF NOT EXISTS idx_login_events_timestamp   ON login_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_login_events_result      ON login_events(result);
CREATE INDEX IF NOT EXISTS idx_login_events_ip_address  ON login_events(ip_address);

-- ======================= CLEANUP & CAS FUNCTIONS =======================

-- Cleanup expired nonces (giữ thêm 1h sau khi hết hạn cho mục đích debug)
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

-- Cleanup login events cũ (mặc định giữ 90 ngày)
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

-- Revoke session đã hết hạn (đặt revoked_at)
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS integer AS $$
DECLARE
  updated_count integer;
BEGIN
  UPDATE sessions
     SET revoked_at = now()
   WHERE expires_at < now()
     AND revoked_at IS NULL;
  GET DIAGNOSTICS updated_count = ROW_COUNT;
  RETURN updated_count;
END;
$$ LANGUAGE plpgsql;

-- CAS consume nonce theo đúng scope; trả TRUE nếu consume thành công
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

-- ======================= COMMENTS =======================
COMMENT ON TABLE  auth_nonces IS 'One-time nonces for SIWE authentication flow';
COMMENT ON COLUMN auth_nonces.nonce      IS 'Unique cryptographically secure nonce value';
COMMENT ON COLUMN auth_nonces.account_id IS 'Ethereum account address (lowercase 0x...)';
COMMENT ON COLUMN auth_nonces.domain     IS 'Domain requesting authentication (from SIWE message)';
COMMENT ON COLUMN auth_nonces.chain_id   IS 'CAIP-2 identifier (e.g., eip155:1)';

COMMENT ON TABLE  sessions IS 'User sessions after successful SIWE verification';
COMMENT ON COLUMN sessions.user_id       IS 'References users table in user service';
COMMENT ON COLUMN sessions.refresh_hash  IS 'HMAC/SHA-256 hash of refresh token';
COMMENT ON COLUMN sessions.device_id     IS 'Optional device fingerprint for multi-device tracking';
COMMENT ON COLUMN sessions.collection_intent_context IS 'Optional JSONB storing collection creation context for auth-to-collection flow';

COMMENT ON TABLE  login_events IS 'Audit log of all authentication attempts';
COMMENT ON COLUMN login_events.result    IS 'Authentication result enum';
COMMENT ON COLUMN login_events.error_message IS 'Detailed error message if failed';

COMMIT;
