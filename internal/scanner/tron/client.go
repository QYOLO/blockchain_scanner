package tron

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	nodeURL string
	client  *http.Client
}

// TronBlock ETH JSON-RPC API返回的区块数据结构
type TronBlock struct {
	Number       string        `json:"number"`       // 区块高度（16进制）
	Hash         string        `json:"hash"`         // 区块哈希
	ParentHash   string        `json:"parentHash"`   // 父区块哈希
	Timestamp    string        `json:"timestamp"`    // 时间戳（16进制）
	Transactions []Transaction `json:"transactions"` // 交易列表
}

type Transaction struct {
	Hash        string `json:"hash"`        // 交易哈希
	From        string `json:"from"`        // 发送方地址
	To          string `json:"to"`          // 接收方地址
	Value       string `json:"value"`       // 交易金额（16进制）
	Input       string `json:"input"`       // 输入数据
	BlockNumber string `json:"blockNumber"` // 区块高度（16进制）
	BlockHash   string `json:"blockHash"`   // 区块哈希
	Timestamp   string `json:"timestamp"`   // 时间戳（16进制）
}

// Log 事件日志结构
type Log struct {
	Address string   `json:"address"`
	Topics  []string `json:"topics"`
	Data    string   `json:"data"`
}

func NewClient(nodeURL string) *Client {
	return &Client{
		nodeURL: nodeURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetBlockByNum(ctx context.Context, blockNum int64) (*TronBlock, error) {
	// 构建 JSON-RPC 请求体
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBlockByNumber",
		"params": []interface{}{
			fmt.Sprintf("0x%x", blockNum), // 转换为16进制
			true,                          // 获取完整的交易信息
		},
		"id": 1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", c.nodeURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// 首先解析为通用的 JSON-RPC 响应结构
	var jsonRPCResp struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &jsonRPCResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %v, body: %s", err, string(body))
	}

	// 检查是否有错误
	if jsonRPCResp.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error: code=%d, message=%s",
			jsonRPCResp.Error.Code, jsonRPCResp.Error.Message)
	}

	// 解析区块数据
	var block TronBlock
	if err := json.Unmarshal(jsonRPCResp.Result, &block); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block data: %v, data: %s", err, string(jsonRPCResp.Result))
	}

	return &block, nil
}

// GetTransactionInfo 获取交易的详细信息
func (c *Client) GetTransactionInfo(ctx context.Context, txID string) (map[string]interface{}, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/wallet/gettransactioninfobyid", c.nodeURL)

	// 构建请求体
	reqBody := map[string]string{
		"value": txID,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json")

	// 发送请求
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v, body: %s", err, string(body))
	}

	return result, nil
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

// IsValidTronAddress 检查是否是有效的TRON地址
func IsValidTronAddress(address string) bool {
	// 移除可能的0x前缀
	address = strings.TrimPrefix(address, "0x")
	// 确保有41前缀
	if !strings.HasPrefix(address, "41") {
		return false
	}
	// 移除41前缀后应该正好是40个字符（20字节的十六进制表示）
	address = strings.TrimPrefix(address, "41")
	if len(address) != 40 {
		return false
	}
	// 验证是否是有效的十六进制字符串
	_, err := hex.DecodeString(address)
	return err == nil
}

// ConvertToTronAddress 将以太坊格式地址转换为TRON格式
func ConvertToTronAddress(ethAddress string) string {
	// 移除0x前缀
	addr := strings.TrimPrefix(ethAddress, "0x")
	// 添加41前缀
	return "41" + addr
}

// ConvertToHexAddress 将TRON格式地址转换为以太坊格式
func ConvertToHexAddress(tronAddress string) string {
	// 移除41前缀
	addr := strings.TrimPrefix(tronAddress, "41")
	// 添加0x前缀
	return "0x" + addr
}

// NormalizeAddress 标准化地址格式
func NormalizeAddress(address string) string {
	// 移除0x前缀
	address = strings.TrimPrefix(address, "0x")
	// 移除41前缀
	address = strings.TrimPrefix(address, "41")

	// 如果地址超过40个字符，只取最后40个字符
	if len(address) > 40 {
		address = address[len(address)-40:]
	}

	// 添加41前缀并转换为大写
	return "41" + strings.ToUpper(address)
}

// HexToInt64 将十六进制字符串转换为int64
func HexToInt64(hex string) int64 {
	if strings.HasPrefix(hex, "0x") {
		hex = hex[2:]
	}
	n := new(big.Int)
	n.SetString(hex, 16)
	return n.Int64()
}

// HexToUint64 将十六进制字符串转换为uint64
func HexToUint64(hex string) uint64 {
	if strings.HasPrefix(hex, "0x") {
		hex = hex[2:]
	}
	n := new(big.Int)
	n.SetString(hex, 16)
	return n.Uint64()
}
