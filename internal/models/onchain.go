package models

type OnchainTransaction struct {
	Hash        string `json:"hash"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	Input       string `json:"input"`
	BlockNumber string `json:"blockNumber"`
	BlockHash   string `json:"blockHash"`
	Timestamp   string `json:"timestamp"`
}

type OnchainLog struct {
	TxHash    string   `json:"transactionHash"`
	LogIndex  string   `json:"logIndex"`
	Address   string   `json:"address"`
	Topics    []string `json:"topics"`
	Data      string   `json:"data"`
	BlockHash string   `json:"blockHash"`
}
