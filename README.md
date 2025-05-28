# Blockchain Scanner

一个基于Go语言的区块链交易数据扫描器，支持实时监控TRON链上的代币转账交易，并将数据保存到MySQL数据库中。

## 功能特点

- 支持TRON链的交易数据扫描
- 支持TRC20代币转账事件解析
- 双向交易记录存储
- 断点续扫功能
- 并发处理和错误重试机制
- 完善的日志记录

## 系统要求

- Go 1.24+
- MySQL 5.7+
- Redis 6.0+

## 安装

1. 克隆项目：

```bash
git clone https://github.com/yourusername/blockchain_scanner.git
cd blockchain_scanner
```

2. 安装依赖：

```bash
go mod download
```

3. 配置文件：

复制配置文件模板并修改：

```bash
cp config/config.yaml.example config/config.yaml
```

修改 `config.yaml` 中的配置：

```yaml
mysql:
  host: "127.0.0.1"
  port: 3306
  database: "blockchain_data"
  username: "your_username"
  password: "your_password"

redis:
  host: "127.0.0.1"
  port: 6379
  password: ""
  db: 0

chains:
  - chain: "tron"
    node_url: "https://api.zan.top/node/v1/tron/mainnet/your_api_key/jsonrpc"
    api_key: "your_api_key"
    enabled: true

tokens:
  - chain: "tron"
    contract_address: "your_contract_address"
    symbol: "USDT"
    decimals: 18
    token_type: "trc20"
```

4. 编译：

```bash
go build -o scanner cmd/scanner/main.go
```

## 运行

```bash
./scanner
```

## 数据库表结构

### transactions 表

```sql
CREATE TABLE transactions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    chain VARCHAR(20) NOT NULL,
    contract_address VARCHAR(66) NOT NULL,
    token_symbol VARCHAR(10) NOT NULL,
    block BIGINT NOT NULL,
    log_index INT NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    timestamp DATETIME NOT NULL,
    from_address VARCHAR(66) NOT NULL,
    to_address VARCHAR(66) NOT NULL,
    amount BIGINT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    INDEX idx_chain (chain),
    INDEX idx_contract (contract_address),
    INDEX idx_block (block),
    INDEX idx_tx_hash (tx_hash),
    INDEX idx_timestamp (timestamp),
    INDEX idx_from (from_address),
    INDEX idx_to (to_address)
);
```

### doris_transactions 表

```sql
CREATE TABLE doris_transactions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    own_address VARCHAR(66) NOT NULL,
    timestamp DATETIME NOT NULL,
    counterparty_address VARCHAR(66) NOT NULL,
    amount BIGINT NOT NULL,
    block BIGINT NOT NULL,
    log_index INT NOT NULL,
    tx_hash VARCHAR(66) NOT NULL,
    direction TINYINT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE KEY uk_tx (own_address, tx_hash, log_index, direction),
    INDEX idx_own_address (own_address),
    INDEX idx_timestamp (timestamp),
    INDEX idx_counterparty (counterparty_address),
    INDEX idx_block (block),
    INDEX idx_tx_hash (tx_hash)
);
```

## 配置说明

### 扫描器配置

- `start_block`: 起始区块高度
- `batch_size`: 每批处理的区块数量
- `concurrent_workers`: 并发工作线程数
- `retry_times`: 失败重试次数
- `retry_interval`: 重试间隔（秒）

### 链配置

- `chain`: 链名称
- `node_url`: 节点URL
- `api_key`: API密钥
- `enabled`: 是否启用

### 代币配置

- `chain`: 链名称
- `contract_address`: 代币合约地址
- `symbol`: 代币符号
- `decimals`: 代币精度
- `token_type`: 代币类型

## 贡献

欢迎提交 Issue 和 Pull Request。

## 许可证

MIT License 