
CREATE TABLE IF NOT EXISTS indexer_checkpoints (
    chain_id        text PRIMARY KEY,               -- ví dụ "eip155:1"
    last_block      bigint NOT NULL,                -- block height đã xử lý tới
    last_block_hash text,                           -- hash của block đó
    previous_block_hash text,                       -- hash của block trước đó (for continuity check)
    safe_block      bigint,                         -- safe block for reorg protection (64 blocks back)
    safe_block_hash text,                           -- hash của safe block
    reorg_detected_count integer DEFAULT 0,         -- số lần phát hiện reorg
    last_reorg_at   timestamptz,                    -- thời điểm reorg cuối cùng
    updated_at      timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE indexer_checkpoints IS 'Lưu checkpoint của indexer cho mỗi chain (1 row/chain)';
COMMENT ON COLUMN indexer_checkpoints.chain_id IS 'CAIP-2 Chain ID (vd: eip155:1)';
COMMENT ON COLUMN indexer_checkpoints.last_block IS 'Block height cuối cùng đã xử lý';
COMMENT ON COLUMN indexer_checkpoints.last_block_hash IS 'Hash của block cuối cùng';
COMMENT ON COLUMN indexer_checkpoints.updated_at IS 'Lần cuối checkpoint được cập nhật';

-- =========================
-- Indexes
-- =========================

-- Truy vấn checkpoint theo chain (đã có qua PRIMARY KEY)
-- PK mặc định sẽ tạo index B-Tree trên chain_id

-- Sắp xếp / monitoring theo block number (ai cao nhất)
CREATE INDEX IF NOT EXISTS idx_idxcp_last_block
    ON indexer_checkpoints(last_block DESC);

-- Sắp xếp theo thời gian cập nhật (dashboard/monitoring)
CREATE INDEX IF NOT EXISTS idx_idxcp_updated_at
    ON indexer_checkpoints(updated_at DESC);

-- Covering index: hữu ích cho dashboard health check
-- (chain_id + last_block + updated_at) → chỉ cần index scan, không phải quay lại bảng
CREATE INDEX IF NOT EXISTS idx_idxcp_health
    ON indexer_checkpoints(chain_id, updated_at DESC, last_block DESC);

-- =========================
-- Chain Reorganization History
-- =========================
CREATE TABLE IF NOT EXISTS reorg_history (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    chain_id        text NOT NULL,
    detected_at     timestamptz NOT NULL DEFAULT now(),
    fork_block      bigint NOT NULL,                -- Block where fork detected
    old_chain_head  bigint NOT NULL,                -- Previous chain head
    new_chain_head  bigint NOT NULL,                -- New chain head after reorg
    old_block_hash  text NOT NULL,                  -- Hash of old block
    new_block_hash  text NOT NULL,                  -- Hash of new block
    affected_blocks integer NOT NULL,               -- Number of blocks affected
    rollback_to     bigint NOT NULL,                -- Block we rolled back to
    data_affected   jsonb,                          -- JSON data about affected NFTs/collections
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_reorg_chain ON reorg_history(chain_id, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_reorg_detected ON reorg_history(detected_at DESC);
