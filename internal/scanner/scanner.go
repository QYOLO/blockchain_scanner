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

	"blockchain_scanner/internal/utils"

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
	logger        *zap.Logger
	db            *gorm.DB
	config        *Config
	tokenConfig   *TokenConfig
	client        *tron.Client
	wg            sync.WaitGroup
	stopChan      chan struct{}
	progressStore *storage.ProgressStore // 进度存储
}

func NewBlockScanner(logger *zap.Logger, db *gorm.DB, config *Config, tokenConfig *TokenConfig, progressStore *storage.ProgressStore) *BlockScanner {
	return &BlockScanner{
		logger:        logger,
		db:            db,
		config:        config,
		tokenConfig:   tokenConfig,
		client:        tron.NewClient(config.NodeURL),
		stopChan:      make(chan struct{}),
		progressStore: progressStore,
	}
}

func (s *BlockScanner) Start(ctx context.Context) error {
	s.logger.Info("Starting block scanner", zap.Int64("from_block", s.config.StartBlock))

	// 启动时从进度存储读取断点
	if s.progressStore != nil {
		if progress, err := s.progressStore.GetProgress(ctx, s.config.Chain); err == nil && progress > 0 {
			s.config.StartBlock = progress
			s.logger.Info("Resuming from saved progress", zap.Int64("block", progress))
		}
	}

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

// 批量插入方法：一次性插入所有数据
func batchInsertTransactions(db *gorm.DB, txs []*storage.Transaction) error {
	if len(txs) == 0 {
		return nil
	}
	return db.Create(&txs).Error
}

// fetchBlockAndLogs 获取指定区块的区块数据和日志，并返回 logIndexMap 及区块时间戳。
func (s *BlockScanner) fetchBlockAndLogs(ctx context.Context, blockNum int64) (*tron.TronBlock, map[string][]int32, time.Time, error) {
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
		return nil, nil, time.Time{}, fmt.Errorf("failed to get block after %d attempts: %v", s.config.RetryTimes, err)
	}

	timestamp := time.Unix(utils.HexToInt64(block.Timestamp), 0)

	// 获取本区块所有log，建立tx_hash到log_index的映射
	filter := map[string]interface{}{
		"fromBlock": fmt.Sprintf("0x%x", blockNum),
		"toBlock":   fmt.Sprintf("0x%x", blockNum+1),
	}
	logs, err := s.client.GetLogs(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to get logs for block", zap.Int64("block", blockNum), zap.Error(err))
		return nil, nil, time.Time{}, err
	}
	logIndexMap := make(map[string][]int32)
	for _, log := range logs {
		if log.TxHash == "" || log.LogIndex == "" {
			continue
		}
		idx, ok := new(big.Int).SetString(strings.TrimPrefix(log.LogIndex, "0x"), 16)
		if !ok {
			continue
		}
		logIndexMap[log.TxHash] = append(logIndexMap[log.TxHash], int32(idx.Int64()))
	}
	return block, logIndexMap, timestamp, nil
}

// parseTransactions 解析区块内所有链上交易，批量转换为数据库 Transaction 和 TronUsdtTransaction 记录。
func (s *BlockScanner) parseTransactions(block *tron.TronBlock, logIndexMap map[string][]int32, timestamp time.Time) ([]*storage.Transaction, []*storage.TronUsdtTransaction) {
	var txRecords []*storage.Transaction
	var tronRecords []*storage.TronUsdtTransaction

	for _, tx := range block.Transactions {
		params := TxConvertParams{
			Tx:          tx,
			LogIndexMap: logIndexMap,
			Block:       block,
			Timestamp:   timestamp,
		}
		txRecord, tronPair := s.convertToDBRecords(params)
		if txRecord != nil {
			txRecords = append(txRecords, txRecord)
		}
		if len(tronPair) > 0 {
			tronRecords = append(tronRecords, tronPair...)
		}
	}
	return txRecords, tronRecords
}

// batchInsertAll 分批批量插入所有交易和双向记录到数据库。
func (s *BlockScanner) batchInsertAll(txRecords []*storage.Transaction, tronRecords []*storage.TronUsdtTransaction, blockNum int64) error {
	if err := bulkInsertTransactionsInBatches(s.db, txRecords, s.config.Chain, s.tokenConfig.Symbol, blockNum, s.logger); err != nil {
		s.logger.Error("Failed to batch insert transactions", zap.Int64("block", blockNum), zap.Error(err))
		return err
	}
	if err := bulkInsertTronUsdtTransactionsInBatches(s.db, tronRecords, s.config.Chain, s.tokenConfig.Symbol, blockNum, s.logger); err != nil {
		s.logger.Error("Failed to batch insert tron_usdt_transactions", zap.Int64("block", blockNum), zap.Error(err))
		return err
	}
	return nil
}

// processBlock 处理单个区块的主流程，调度各子步骤。
func (s *BlockScanner) processBlock(ctx context.Context, blockNum int64) error {
	block, logIndexMap, timestamp, err := s.fetchBlockAndLogs(ctx, blockNum)
	if err != nil {
		return err
	}
	txRecords, tronRecords := s.parseTransactions(block, logIndexMap, timestamp)
	if err := s.batchInsertAll(txRecords, tronRecords, blockNum); err != nil {
		return err
	}
	s.logger.Info("Block processed with batch insert", zap.Int64("block", blockNum), zap.Int("tx_count", len(txRecords)), zap.Int("tron_record_count", len(tronRecords)))

	// 处理成功后保存进度到 Redis
	if s.progressStore != nil {
		if err := s.progressStore.SaveProgress(ctx, s.config.Chain, blockNum); err != nil {
			s.logger.Warn("Failed to save progress", zap.Error(err))
		}
	}
	return nil
}
