package models

type BaseBlock struct {
	Number     string `json:"number"`
	Hash       string `json:"hash"`
	ParentHash string `json:"parentHash"`
	Timestamp  string `json:"timestamp"`
}

type OnchainBlock struct {
	BaseBlock
	Transactions []OnchainTransaction `json:"transactions"`
}
