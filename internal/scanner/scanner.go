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

// 批量插入方法：一次性插入所有数据
func batchInsertTransactions(db *gorm.DB, txs []*storage.Transaction) error {
	if len(txs) == 0 {
		return nil
	}
	return db.Create(&txs).Error
}

// 分批批量插入方法：每批 batchSize 条
func bulkInsertTransactionsInBatches(db *gorm.DB, txs []*storage.Transaction, batchSize int) error {
	total := len(txs)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := txs[i:end]
		if err := db.Create(&batch).Error; err != nil {
			return err
		}
	}
	return nil
}

// 分批批量插入 TronUsdtTransaction
func bulkInsertTronUsdtTransactionsInBatches(db *gorm.DB, txs []*storage.TronUsdtTransaction, batchSize int) error {
	total := len(txs)
	for i := 0; i < total; i += batchSize {
		end := i + batchSize
		if end > total {
			end = total
		}
		batch := txs[i:end]
		if err := db.Create(&batch).Error; err != nil {
			return err
		}
	}
	return nil
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

	timestamp := time.Unix(tron.HexToInt64(block.Timestamp), 0)

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

// TxConvertParams 用于聚合链上交易转换为数据库模型所需的所有参数。
type TxConvertParams struct {
	Tx          tron.Transaction
	LogIndexMap map[string][]int32
	Block       *tron.TronBlock
	Timestamp   time.Time
}

// parseTrc20Transfer 负责解析和校验 TRC20 transfer 交易，返回标准化后的 from、to、amount，失败返回 error。
func (s *BlockScanner) parseTrc20Transfer(tx tron.Transaction, logIndexMap map[string][]int32) (from, to string, amount *big.Int, logIndex int32, err error) {
	if !strings.HasPrefix(tx.Input, "0xa9059cbb") {
		return "", "", nil, 0, fmt.Errorf("not a TRC20 transfer method")
	}
	if !utils.IsTargetContract(tx.To, s.tokenConfig.ContractAddress, "tron") {
		return "", "", nil, 0, fmt.Errorf("not target contract")
	}
	fromAddr, toAddr, amt, err := tron.ParseTRC20Transfer(tx.Input)
	if err != nil {
		return "", "", nil, 0, err
	}
	if fromAddr == "" {
		fromAddr = utils.NormalizeAddress(tx.From, "tron")
	} else {
		fromAddr = utils.NormalizeAddress(fromAddr, "tron")
	}
	toAddr = utils.NormalizeAddress(toAddr, "tron")
	if !tron.IsValidTronAddress(fromAddr) || !tron.IsValidTronAddress(toAddr) {
		return "", "", nil, 0, fmt.Errorf("invalid address format: from=%s, to=%s", fromAddr, toAddr)
	}
	logIndex = utils.GetLogIndex(logIndexMap, tx.Hash)
	return fromAddr, toAddr, amt, logIndex, nil
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

// convertToDBRecords 将一笔链上交易转换为一条 Transaction 记录和两条 TronUsdtTransaction 记录（出账/入账）。
// 不满足条件时返回 nil, nil。
func (s *BlockScanner) convertToDBRecords(params TxConvertParams) (*storage.Transaction, []*storage.TronUsdtTransaction) {
	tx := params.Tx
	logIndexMap := params.LogIndexMap
	block := params.Block
	timestamp := params.Timestamp

	from, to, amount, logIndex, err := s.parseTrc20Transfer(tx, logIndexMap)
	if err != nil {
		// 不是TRC20转账或校验失败直接跳过
		return nil, nil
	}
	adjustedAmount := utils.AdjustAmount(amount, s.tokenConfig.Decimals)
	blockNum := tron.HexToInt64(block.Number)
	txRecord := &storage.Transaction{
		Chain:           s.config.Chain,
		ContractAddress: s.tokenConfig.ContractAddress,
		TokenSymbol:     s.tokenConfig.Symbol,
		Block:           blockNum,
		LogIndex:        logIndex,
		TxHash:          tx.Hash,
		FromAddress:     from,
		ToAddress:       to,
		Amount:          adjustedAmount.Int64(),
		Timestamp:       timestamp,
	}
	tronPair := []*storage.TronUsdtTransaction{
		{
			OwnAddress:          from,
			Timestamp:           timestamp,
			CounterpartyAddress: to,
			Amount:              adjustedAmount.Int64(),
			Block:               blockNum,
			LogIndex:            logIndex,
			TxHash:              tx.Hash,
			Direction:           1, // 出账
		},
		{
			OwnAddress:          to,
			Timestamp:           timestamp,
			CounterpartyAddress: from,
			Amount:              adjustedAmount.Int64(),
			Block:               blockNum,
			LogIndex:            logIndex,
			TxHash:              tx.Hash,
			Direction:           0, // 进账
		},
	}
	return txRecord, tronPair
}

// batchInsertAll 分批批量插入所有交易和双向记录到数据库。
func (s *BlockScanner) batchInsertAll(txRecords []*storage.Transaction, tronRecords []*storage.TronUsdtTransaction, blockNum int64) error {
	const batchSize = 500
	if err := bulkInsertTransactionsInBatches(s.db, txRecords, batchSize); err != nil {
		s.logger.Error("Failed to batch insert transactions", zap.Int64("block", blockNum), zap.Error(err))
		return err
	}
	if err := bulkInsertTronUsdtTransactionsInBatches(s.db, tronRecords, batchSize); err != nil {
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
	return nil
}
