package scanner

import (
	"blockchain_scanner/internal/client"
	"blockchain_scanner/internal/models"
	"context"
	"encoding/json"
	"fmt"
)

type EthClient struct {
	rpcClient *client.RPCClient
}

func NewEthClient(nodeURL string) *EthClient {
	return &EthClient{
		rpcClient: client.NewRPCClient(nodeURL),
	}
}

func (c *EthClient) GetBlockByNum(ctx context.Context, blockNum int64) (*models.OnchainBlock, error) {
	params := []interface{}{
		fmt.Sprintf("0x%x", blockNum),
		true,
	}
	resp, err := c.rpcClient.DoJSONRPC(ctx, "eth_getBlockByNumber", params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}
	var block models.OnchainBlock
	if err := json.Unmarshal(resp.Result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block data: %v, data: %s", err, string(resp.Result))
	}
	return &block, nil
}

func (c *EthClient) GetLogs(ctx context.Context, filter map[string]interface{}) ([]models.OnchainLog, error) {
	params := []interface{}{filter}
	resp, err := c.rpcClient.DoJSONRPC(ctx, "eth_getLogs", params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}
	var logs []models.OnchainLog
	if err := json.Unmarshal(resp.Result, &logs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %v, data: %s", err, string(resp.Result))
	}
	return logs, nil
}
