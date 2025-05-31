package client

import (
	"blockchain_scanner/internal/models"
	"context"
)

type BlockchainClient interface {
	GetBlockByNum(ctx context.Context, blockNum int64) (*models.OnchainBlock, error)
	GetLogs(ctx context.Context, filter map[string]interface{}) ([]models.OnchainLog, error)
}
