
CREATE TABLE IF NOT EXISTS indexer_checkpoints (
    chain_id        text PRIMARY KEY,               -- ví dụ "eip155:1"
    last_block      bigint NOT NULL,                -- block height đã xử lý tới
    last_block_hash text,                           -- hash của block đó
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
