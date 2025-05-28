package scanner

import (
	"blockchain_scanner/internal/scanner/tron"
	"blockchain_scanner/internal/storage"
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Config struct {
	StartBlock        int64  `mapstructure:"start_block"`
	BatchSize         int    `mapstructure:"batch_size"`
	ConcurrentWorkers int    `mapstructure:"concurrent_workers"`
	RetryTimes        int    `mapstructure:"retry_times"`
	RetryInterval     int    `mapstructure:"retry_interval"`
	ScanInterval      int    `mapstructure:"scan_interval"`
	NodeURL           string `mapstructure:"node_url"`
	Chain             string `mapstructure:"chain"`
}

type TokenConfig struct {
	Chain           string `mapstructure:"chain"`
	ContractAddress string `mapstructure:"contract_address"`
	Symbol          string `mapstructure:"symbol"`
	Decimals        int    `mapstructure:"decimals"`
}

type BlockScanner struct {
	logger      *zap.Logger
	db          *gorm.DB
	config      *Config
	tokenConfig *TokenConfig
	client      *tron.Client
	wg          sync.WaitGroup
	stopChan    chan struct{}
}

func NewBlockScanner(logger *zap.Logger, db *gorm.DB, config *Config, tokenConfig *TokenConfig) *BlockScanner {
	return &BlockScanner{
		logger:      logger,
		db:          db,
		config:      config,
		tokenConfig: tokenConfig,
		client:      tron.NewClient(config.NodeURL),
		stopChan:    make(chan struct{}),
	}
}

func (s *BlockScanner) Start(ctx context.Context) error {
	s.logger.Info("Starting block scanner", zap.Int64("from_block", s.config.StartBlock))

	// 创建任务通道
	taskChan := make(chan int64, s.config.ConcurrentWorkers)

	// 启动工作协程
	for i := 0; i < s.config.ConcurrentWorkers; i++ {
		s.wg.Add(1)
		go func(workerID int) {
			defer s.wg.Done()
			s.logger.Info("Worker started", zap.Int("worker_id", workerID))
			s.worker(ctx, taskChan)
		}(i)
	}

	// 启动区块分发协程
	go s.blockDispatcher(ctx, taskChan)

	return nil
}

func (s *BlockScanner) Stop() {
	close(s.stopChan)
	s.wg.Wait()
}

func (s *BlockScanner) worker(ctx context.Context, taskChan <-chan int64) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case blockNum, ok := <-taskChan:
			if !ok {
				return
			}

			if err := s.processBlock(ctx, blockNum); err != nil {
				s.logger.Error("Failed to process block",
					zap.Int64("block_number", blockNum),
					zap.Error(err))
				continue
			}
		}
	}
}

func (s *BlockScanner) blockDispatcher(ctx context.Context, taskChan chan<- int64) {
	currentBlock := s.config.StartBlock

	// 设置默认扫描间隔为3秒
	scanInterval := s.config.ScanInterval
	if scanInterval <= 0 {
		scanInterval = 3
	}

	ticker := time.NewTicker(time.Duration(scanInterval) * time.Second)
	defer ticker.Stop()

	s.logger.Info("Block dispatcher started",
		zap.Int64("start_block", currentBlock),
		zap.Int("scan_interval", scanInterval))

	for {
		select {
		case <-ctx.Done():
			close(taskChan)
			return
		case <-s.stopChan:
			close(taskChan)
			return
		case <-ticker.C:
			// 分发区块任务
			for i := 0; i < s.config.BatchSize; i++ {
				select {
				case taskChan <- currentBlock:
					s.logger.Debug("Dispatched block",
						zap.Int64("block_number", currentBlock))
					currentBlock++
				case <-ctx.Done():
					close(taskChan)
					return
				case <-s.stopChan:
					close(taskChan)
					return
				}
			}
		}
	}
}

func (s *BlockScanner) processBlock(ctx context.Context, blockNum int64) error {
	// 获取区块数据，带重试
	var block *tron.TronBlock
	var err error
	for i := 0; i < s.config.RetryTimes; i++ {
		block, err = s.client.GetBlockByNum(ctx, blockNum)
		if err == nil {
			break
		}
		s.logger.Warn("Failed to get block, retrying...",
			zap.Int64("block", blockNum),
			zap.Error(err),
			zap.Int("attempt", i+1))
		time.Sleep(time.Duration(s.config.RetryInterval) * time.Second)
	}
	if err != nil {
		return fmt.Errorf("failed to get block after %d attempts: %v", s.config.RetryTimes, err)
	}

	s.logger.Debug("Processing block",
		zap.Int64("block", blockNum),
		zap.Int("tx_count", len(block.Transactions)))

	// 解析区块时间戳（从16进制转换）
	timestamp := time.Unix(tron.HexToInt64(block.Timestamp), 0)

	// 处理区块中的每个交易
	for _, tx := range block.Transactions {
		// 检查是否是合约调用（通过input数据判断）
		if !strings.HasPrefix(tx.Input, "0xa9059cbb") { // transfer方法的签名
			continue
		}

		// 标准化合约地址（移除0x前缀并转换为大写）
		contractAddress := strings.ToUpper(strings.TrimPrefix(tx.To, "0x"))
		targetAddress := strings.ToUpper(strings.TrimPrefix(s.tokenConfig.ContractAddress, "0x"))

		// 如果合约地址没有41前缀，添加它
		if !strings.HasPrefix(contractAddress, "41") {
			contractAddress = "41" + contractAddress
		}

		// 确保目标地址有41前缀
		if !strings.HasPrefix(targetAddress, "41") {
			targetAddress = "41" + targetAddress
		}

		// 检查是否是目标代币的转账
		if contractAddress != targetAddress {
			s.logger.Debug("Skipping non-target token transfer",
				zap.String("tx_hash", tx.Hash),
				zap.String("contract_address", contractAddress),
				zap.String("target_address", targetAddress))
			continue
		}

		s.logger.Debug("Found target token transfer",
			zap.String("tx_hash", tx.Hash),
			zap.String("contract_address", contractAddress),
			zap.String("input", tx.Input))

		// 解析转账事件
		from, to, amount, err := tron.ParseTRC20Transfer(tx.Input)
		if err != nil {
			s.logger.Debug("Failed to parse transfer data",
				zap.String("tx_hash", tx.Hash),
				zap.Error(err))
			continue
		}

		// 使用交易的发送者作为from地址
		if from == "" {
			from = strings.ToUpper(strings.TrimPrefix(tx.From, "0x"))
			if !strings.HasPrefix(from, "41") {
				from = "41" + from
			}
		}

		// 标准化地址格式
		from = tron.NormalizeAddress(from)
		to = tron.NormalizeAddress(to)

		// 验证地址格式
		if !tron.IsValidTronAddress(from) || !tron.IsValidTronAddress(to) {
			s.logger.Warn("Invalid address format",
				zap.String("tx_hash", tx.Hash),
				zap.String("from", from),
				zap.String("to", to))
			continue
		}

		// 调整金额精度
		decimals := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(s.tokenConfig.Decimals)), nil)
		adjustedAmount := new(big.Int).Div(amount, decimals)

		s.logger.Debug("Processing transfer",
			zap.String("tx_hash", tx.Hash),
			zap.String("from", from),
			zap.String("to", to),
			zap.String("raw_amount", amount.String()),
			zap.String("adjusted_amount", adjustedAmount.String()))

		// 创建交易记录
		txRecord := &storage.Transaction{
			Chain:           s.config.Chain,
			ContractAddress: s.tokenConfig.ContractAddress,
			TokenSymbol:     s.tokenConfig.Symbol,
			Block:           blockNum,
			LogIndex:        0, // TODO: 从事件日志中获取正确的LogIndex
			TxHash:          tx.Hash,
			FromAddress:     from,
			ToAddress:       to,
			Amount:          adjustedAmount.Int64(),
			Timestamp:       timestamp,
		}

		// 创建双向记录
		tronRecords := []*storage.TronUsdtTransaction{
			{
				OwnAddress:          from,
				Timestamp:           timestamp,
				CounterpartyAddress: to,
				Amount:              adjustedAmount.Int64(),
				Block:               blockNum,
				LogIndex:            0, // TODO: 从事件日志中获取正确的LogIndex
				TxHash:              tx.Hash,
				Direction:           1, // 出账
			},
			{
				OwnAddress:          to,
				Timestamp:           timestamp,
				CounterpartyAddress: from,
				Amount:              adjustedAmount.Int64(),
				Block:               blockNum,
				LogIndex:            0, // TODO: 从事件日志中获取正确的LogIndex
				TxHash:              tx.Hash,
				Direction:           0, // 进账
			},
		}

		// 在事务中保存记录
		err = s.db.Transaction(func(tx *gorm.DB) error {
			// 保存原始交易记录
			if err := tx.Create(txRecord).Error; err != nil {
				if !strings.Contains(err.Error(), "Duplicate entry") {
					return fmt.Errorf("failed to save transaction: %v", err)
				}
				// 如果是重复记录，则忽略
				return nil
			}

			// 保存双向记录
			for _, record := range tronRecords {
				if err := tx.Create(record).Error; err != nil {
					return fmt.Errorf("failed to save tron record: %v", err)
				}
			}

			return nil
		})

		if err != nil {
			s.logger.Error("Failed to save records",
				zap.String("tx_hash", tx.Hash),
				zap.Error(err))
			continue
		}

		s.logger.Info("Transaction processed",
			zap.Int64("block", blockNum),
			zap.String("hash", tx.Hash),
			zap.String("from", from),
			zap.String("to", to),
			zap.String("amount", adjustedAmount.String()))
	}

	return nil
}
