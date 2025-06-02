// internal/scanner/tron/client.go
package tron

import (
	"blockchain_scanner/internal/client"
	"blockchain_scanner/internal/models"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

type Client struct {
	rpcClient *client.RPCClient
}

// TronBlock 继承 models.BaseBlock
type TronBlock struct {
	Number       string                      `json:"number"`
	Hash         string                      `json:"hash"`
	ParentHash   string                      `json:"parentHash"`
	Timestamp    string                      `json:"timestamp"`
	Transactions []models.OnchainTransaction `json:"transactions"`
}

// Transaction 继承 models.BaseTransaction
type Transaction = models.OnchainTransaction

// Log 事件日志结构
type Log struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

// TronLog 结构体，包含TxHash和LogIndex
// 可根据需要扩展其他字段
// 放在文件顶部或合适位置

type TronLog struct {
	TxHash    string   `json:"transactionHash"`
	LogIndex  string   `json:"logIndex"`
	Address   string   `json:"address"`
	Topics    []string `json:"topics"`
	Data      string   `json:"data"`
	BlockHash string   `json:"blockHash"`
	// 可扩展其他字段
}

func NewClient(nodeURL string) *Client {
	return &Client{
		rpcClient: client.NewRPCClient(nodeURL),
	}
}

func (c *Client) GetBlockByNum(ctx context.Context, blockNum int64) (*TronBlock, error) {
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
	var block TronBlock
	if err := json.Unmarshal(resp.Result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block data: %v, data: %s", err, string(resp.Result))
	}
	return &block, nil
}

// GetLogs 通过 Tron JSON-RPC API 获取日志
func (c *Client) GetLogs(ctx context.Context, filter map[string]interface{}) ([]TronLog, error) {
	params := []interface{}{filter}
	resp, err := c.rpcClient.DoJSONRPC(ctx, "eth_getLogs", params)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: code=%d, message=%s", resp.Error.Code, resp.Error.Message)
	}
	var logs []TronLog
	if err := json.Unmarshal(resp.Result, &logs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %v, data: %s", err, string(resp.Result))
	}
	return logs, nil
}

// ParseTRC20Transfer 解析TRC20代币转账事件
func ParseTRC20Transfer(data string) (from, to string, amount *big.Int, err error) {
	// 移除0x前缀
	data = strings.TrimPrefix(data, "0x")

	// 检查最小长度：方法签名(8) + 接收地址(64) + 金额(64) = 136 hex chars
	if len(data) < 136 {
		return "", "", nil, fmt.Errorf("invalid data length: got %d, want at least 136", len(data))
	}

	// 跳过方法签名(8个字符)
	// 解析to地址（第一个参数，从第8个字符开始）
	toParam := data[8+24 : 8+64] // 取32字节参数中的后20字节
	toBytes, err := hex.DecodeString(toParam)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to decode to address: %v, raw: %s", err, toParam)
	}
	// 确保地址正好是20字节
	if len(toBytes) > 20 {
		toBytes = toBytes[len(toBytes)-20:] // 只取最后20字节
	}
	to = fmt.Sprintf("41%x", toBytes)

	// 解析amount（第二个参数）
	amount = new(big.Int)
	amountParam := data[72:136] // 从第72个字符开始，读取64个字符
	if _, ok := amount.SetString(amountParam, 16); !ok {
		return "", "", nil, fmt.Errorf("failed to parse amount: %s", amountParam)
	}

	// 对于transfer方法，from地址是交易的发送者，在交易对象中，而不是在input数据中
	return "", strings.ToUpper(to), amount, nil
}
