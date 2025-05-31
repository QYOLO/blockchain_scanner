package client

import (
	"blockchain_scanner/internal/models"
	"context"
	"encoding/json"
	"fmt"
)

type RPCClient struct {
	HttpClient *HttpClient
}

func NewRPCClient(nodeURL string) *RPCClient {
	return &RPCClient{
		HttpClient: NewHttpClient(nodeURL),
	}
}

func (c *RPCClient) DoJSONRPC(ctx context.Context, method string, params interface{}) (*models.JSONRPCResponse, error) {
	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
		"id":      1,
	}
	resp, err := c.HttpClient.Do(ctx, "POST", "", &RequestOption{Body: reqBody})
	if err != nil {
		return nil, err
	}
	var jsonRPCResp models.JSONRPCResponse
	if err := json.Unmarshal(resp, &jsonRPCResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON-RPC response: %v, body: %s", err, string(resp))
	}
	return &jsonRPCResp, nil
}
