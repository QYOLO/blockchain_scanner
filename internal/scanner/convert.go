package scanner

import (
	"blockchain_scanner/internal/scanner/tron"
	"blockchain_scanner/internal/storage"
	"blockchain_scanner/internal/utils"
	"fmt"
	"math/big"
	"strings"
	"time"
)

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
	if !utils.IsValidTronAddress(fromAddr) || !utils.IsValidTronAddress(toAddr) {
		return "", "", nil, 0, fmt.Errorf("invalid address format: from=%s, to=%s", fromAddr, toAddr)
	}
	logIndex = utils.GetLogIndex(logIndexMap, tx.Hash)
	return fromAddr, toAddr, amt, logIndex, nil
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
	blockNum := utils.HexToInt64(block.Number)
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
