package models

import "encoding/json"

// JSONRPCError 兼容所有链的RPC错误结构
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSONRPCResponse 兼容所有链的RPC响应结构
// Result 字段用 json.RawMessage 以支持对象、数组、字符串等多种类型
// 业务层可根据需要二次解析 Result
//
// 示例：
//
//	var block models.OnchainBlock
//	json.Unmarshal(resp.Result, &block)
//
//	var txs []models.OnchainTransaction
//	json.Unmarshal(resp.Result, &txs)
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}
