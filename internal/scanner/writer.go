package scanner

import (
	"blockchain_scanner/internal/storage"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Writer 接口，便于后续扩展多种写入后端
// type Writer interface {
// 	InsertTransactionsInBatches(...)
// 	InsertTronUsdtTransactionsInBatches(...)
// }

// 批量插入方法：每批加事务，失败重试，日志脱敏
func bulkInsertTransactionsInBatches(db *gorm.DB, txs []*storage.Transaction, chain, token string, blockNum int64, logger *zap.Logger) error {
	total := len(txs)
	for i := 0; i < total; i += BatchInsertSize {
		end := i + BatchInsertSize
		if end > total {
			end = total
		}
		batch := txs[i:end]
		var err error
		for retry := 0; retry < BatchInsertMaxRetry; retry++ {
			err = db.Transaction(func(tx *gorm.DB) error {
				return tx.Create(&batch).Error
			})
			if err == nil {
				break
			}
			logger.Warn("Batch insert failed, retrying...",
				zap.String("chain", chain),
				zap.String("token", token),
				zap.Int64("block", blockNum),
				zap.Int("batch_start", i),
				zap.Int("batch_end", end),
				zap.Int("retry", retry+1),
				zap.Error(err),
				zap.Any("batch_sample", maskTransactions(batch, 2)),
			)
			time.Sleep(time.Duration(BatchInsertRetryInterval) * time.Second)
		}
		if err != nil {
			logger.Error("Batch insert permanently failed",
				zap.String("chain", chain),
				zap.String("token", token),
				zap.Int64("block", blockNum),
				zap.Int("batch_start", i),
				zap.Int("batch_end", end),
				zap.Error(err),
				zap.Any("batch_sample", maskTransactions(batch, 2)),
			)
			// 可选：将失败批次推送到补偿队列
		}
	}
	return nil
}

// TronUsdtTransaction 批量插入同理
func bulkInsertTronUsdtTransactionsInBatches(db *gorm.DB, txs []*storage.TronUsdtTransaction, chain, token string, blockNum int64, logger *zap.Logger) error {
	total := len(txs)
	for i := 0; i < total; i += BatchInsertSize {
		end := i + BatchInsertSize
		if end > total {
			end = total
		}
		batch := txs[i:end]
		var err error
		for retry := 0; retry < BatchInsertMaxRetry; retry++ {
			err = db.Transaction(func(tx *gorm.DB) error {
				return tx.Create(&batch).Error
			})
			if err == nil {
				break
			}
			logger.Warn("Batch insert failed, retrying...",
				zap.String("chain", chain),
				zap.String("token", token),
				zap.Int64("block", blockNum),
				zap.Int("batch_start", i),
				zap.Int("batch_end", end),
				zap.Int("retry", retry+1),
				zap.Error(err),
				// 可选：脱敏输出部分数据
			)
			time.Sleep(time.Duration(BatchInsertRetryInterval) * time.Second)
		}
		if err != nil {
			logger.Error("Batch insert permanently failed",
				zap.String("chain", chain),
				zap.String("token", token),
				zap.Int64("block", blockNum),
				zap.Int("batch_start", i),
				zap.Int("batch_end", end),
				zap.Error(err),
			)
			// 可选：将失败批次推送到补偿队列
		}
	}
	return nil
}
