-- ===== Wallet Service DOWN (matches latest UP) =====

-- 1) Triggers
DROP TRIGGER IF EXISTS trg_wallets_single_primary ON wallets;
DROP TRIGGER IF EXISTS trg_add_approval_history ON approvals;
DROP TRIGGER IF EXISTS trg_approvals_updated_at ON approvals;
DROP TRIGGER IF EXISTS trg_wallets_updated_at ON wallets;

-- 2) Functions
DROP FUNCTION IF EXISTS ensure_single_primary_wallet();
DROP FUNCTION IF EXISTS add_approval_history();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- 3) Indexes

-- approvals_history
DROP INDEX IF EXISTS idx_approvals_history_at;
DROP INDEX IF EXISTS idx_approvals_history_wallet_id;

-- approvals
DROP INDEX IF EXISTS idx_approvals_standard;
DROP INDEX IF EXISTS idx_approvals_operator;
DROP INDEX IF EXISTS idx_approvals_chain_id;
DROP INDEX IF EXISTS idx_approvals_wallet_id;

-- wallets (unique + common)
DROP INDEX IF EXISTS ux_wallets_user_chain_primary;
DROP INDEX IF EXISTS ux_wallets_chain_addr;
DROP INDEX IF EXISTS ux_wallets_account_id;
DROP INDEX IF EXISTS idx_wallets_last_seen_at;
DROP INDEX IF EXISTS idx_wallets_user_id;

-- 4) Tables (reverse order)
DROP TABLE IF EXISTS approvals_history;
DROP TABLE IF EXISTS approvals;
DROP TABLE IF EXISTS wallets;

-- (Optional) Nếu extension chỉ dùng cho schema này:
-- DROP EXTENSION IF EXISTS pgcrypto;
