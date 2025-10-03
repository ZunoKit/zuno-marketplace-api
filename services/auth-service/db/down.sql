BEGIN;

-- Xoá function theo thứ tự ngược
DROP FUNCTION IF EXISTS try_use_nonce(text, text, text, text);
DROP FUNCTION IF EXISTS cleanup_expired_sessions();
DROP FUNCTION IF EXISTS cleanup_old_login_events(integer);
DROP FUNCTION IF EXISTS cleanup_expired_nonces();

-- Xoá index/column được thêm bởi enhanced session context
DROP INDEX IF EXISTS idx_sessions_collection_context;
ALTER TABLE IF EXISTS sessions DROP COLUMN IF EXISTS collection_intent_context;

-- Xoá bảng (indexes/constraints sẽ đi kèm)
DROP TABLE IF EXISTS login_events;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS auth_nonces;

-- Extensions thường để nguyên (tránh ảnh hưởng phần khác của DB)
-- Nếu thật sự muốn dọn:
-- DROP EXTENSION IF EXISTS pgcrypto;
-- DROP EXTENSION IF EXISTS "uuid-ossp";

COMMIT;
