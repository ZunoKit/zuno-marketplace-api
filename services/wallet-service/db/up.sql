-- Create wallets table
CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    account_id VARCHAR(255) NOT NULL,
    address VARCHAR(42) NOT NULL, -- Ethereum addresses are 42 chars (0x + 40 hex chars)
    chain_id VARCHAR(255) NOT NULL, -- CAIP-2 format like "eip155:1"
    type VARCHAR(50) NOT NULL DEFAULT 'eoa', -- "eoa", "contract"
    connector VARCHAR(100) NOT NULL DEFAULT 'unknown', -- "metamask", "walletconnect", etc.
    is_primary BOOLEAN NOT NULL DEFAULT FALSE,
    label VARCHAR(255),
    verified_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT wallets_address_check CHECK (address ~ '^0x[a-fA-F0-9]{40}$'),
    CONSTRAINT wallets_chain_id_check CHECK (chain_id ~ '^[a-zA-Z0-9]+:[0-9]+$')
);

-- Create unique constraint to prevent duplicate wallet links
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallets_user_address_chain 
ON wallets (user_id, address, chain_id);

-- Create index for faster lookups
CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets (user_id);
CREATE INDEX IF NOT EXISTS idx_wallets_address ON wallets (address);
CREATE INDEX IF NOT EXISTS idx_wallets_chain_id ON wallets (chain_id);
CREATE INDEX IF NOT EXISTS idx_wallets_is_primary ON wallets (user_id, is_primary) WHERE is_primary = TRUE;

-- Create approvals table
CREATE TABLE IF NOT EXISTS approvals (
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    chain_id VARCHAR(255) NOT NULL,
    operator VARCHAR(42) NOT NULL, -- Contract address
    standard VARCHAR(20) NOT NULL, -- "erc20", "erc721", "erc1155"
    approved BOOLEAN NOT NULL,
    approved_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    tx_hash VARCHAR(66) NOT NULL, -- Transaction hash (0x + 64 hex chars)
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Constraints
    PRIMARY KEY (wallet_id, chain_id, operator, standard),
    CONSTRAINT approvals_operator_check CHECK (operator ~ '^0x[a-fA-F0-9]{40}$'),
    CONSTRAINT approvals_tx_hash_check CHECK (tx_hash ~ '^0x[a-fA-F0-9]{64}$'),
    CONSTRAINT approvals_standard_check CHECK (standard IN ('erc20', 'erc721', 'erc1155')),
    CONSTRAINT approvals_chain_id_check CHECK (chain_id ~ '^[a-zA-Z0-9]+:[0-9]+$')
);

-- Create indexes for approvals
CREATE INDEX IF NOT EXISTS idx_approvals_wallet_id ON approvals (wallet_id);
CREATE INDEX IF NOT EXISTS idx_approvals_chain_id ON approvals (chain_id);
CREATE INDEX IF NOT EXISTS idx_approvals_operator ON approvals (operator);
CREATE INDEX IF NOT EXISTS idx_approvals_standard ON approvals (standard);

-- Create approvals_history table for audit trail
CREATE TABLE IF NOT EXISTS approvals_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID NOT NULL,
    chain_id VARCHAR(255) NOT NULL,
    operator VARCHAR(42) NOT NULL,
    standard VARCHAR(20) NOT NULL,
    approved BOOLEAN NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT approvals_history_operator_check CHECK (operator ~ '^0x[a-fA-F0-9]{40}$'),
    CONSTRAINT approvals_history_tx_hash_check CHECK (tx_hash ~ '^0x[a-fA-F0-9]{64}$'),
    CONSTRAINT approvals_history_standard_check CHECK (standard IN ('erc20', 'erc721', 'erc1155')),
    CONSTRAINT approvals_history_chain_id_check CHECK (chain_id ~ '^[a-zA-Z0-9]+:[0-9]+$')
);

-- Create indexes for approvals_history
CREATE INDEX IF NOT EXISTS idx_approvals_history_wallet_id ON approvals_history (wallet_id);
CREATE INDEX IF NOT EXISTS idx_approvals_history_at ON approvals_history (at DESC);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to automatically update updated_at
CREATE TRIGGER update_wallets_updated_at 
    BEFORE UPDATE ON wallets 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_approvals_updated_at 
    BEFORE UPDATE ON approvals 
    FOR EACH ROW 
    EXECUTE FUNCTION update_updated_at_column();

-- Create function to automatically add approval history
CREATE OR REPLACE FUNCTION add_approval_history()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO approvals_history (wallet_id, chain_id, operator, standard, approved, tx_hash, at)
    VALUES (NEW.wallet_id, NEW.chain_id, NEW.operator, NEW.standard, NEW.approved, NEW.tx_hash, NOW());
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to automatically add approval history on insert/update
CREATE TRIGGER add_approval_history_trigger
    AFTER INSERT OR UPDATE ON approvals
    FOR EACH ROW
    EXECUTE FUNCTION add_approval_history();

-- Create function to ensure only one primary wallet per user per chain
CREATE OR REPLACE FUNCTION ensure_single_primary_wallet()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.is_primary = TRUE THEN
        -- Unset other primary wallets for the same user and chain
        UPDATE wallets 
        SET is_primary = FALSE, updated_at = NOW()
        WHERE user_id = NEW.user_id 
          AND chain_id = NEW.chain_id 
          AND id != NEW.id 
          AND is_primary = TRUE;
    END IF;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create trigger to ensure single primary wallet
CREATE TRIGGER ensure_single_primary_wallet_trigger
    BEFORE INSERT OR UPDATE ON wallets
    FOR EACH ROW
    EXECUTE FUNCTION ensure_single_primary_wallet();
