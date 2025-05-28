-- 创建交易表
CREATE TABLE IF NOT EXISTS transactions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    chain VARCHAR(20) NOT NULL,
    contract_address VARCHAR(66) NOT NULL,
    token_symbol VARCHAR(10) NOT NULL,
    block BIGINT NOT NULL,
    log_index INT NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    timestamp DATETIME NOT NULL,
    from_address VARCHAR(42) NOT NULL,
    to_address VARCHAR(42) NOT NULL,
    amount BIGINT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME,
    INDEX idx_block (block),
    UNIQUE INDEX idx_tx_hash (tx_hash),
    INDEX idx_from_address (from_address),
    INDEX idx_to_address (to_address),
    INDEX idx_timestamp (timestamp),
    INDEX idx_contract_address (contract_address),
    INDEX idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 创建TRON USDT双向记录表
CREATE TABLE IF NOT EXISTS tron_usdt_transactions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    own_address VARCHAR(42) NOT NULL,
    timestamp DATETIME NOT NULL,
    counterparty_address VARCHAR(42) NOT NULL,
    amount BIGINT NOT NULL,
    block BIGINT NOT NULL,
    log_index INT NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    direction TINYINT NOT NULL COMMENT '0-进，1-出',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME,
    INDEX idx_own_address (own_address),
    INDEX idx_timestamp (timestamp),
    INDEX idx_counterparty_address (counterparty_address),
    INDEX idx_block (block),
    INDEX idx_tx_hash (tx_hash),
    INDEX idx_deleted_at (deleted_at),
    UNIQUE INDEX uk_tx_direction (own_address, tx_hash, log_index, direction)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci; 