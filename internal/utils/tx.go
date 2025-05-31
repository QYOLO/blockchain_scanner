package utils

import (
	"math/big"
	"strings"
)

// NormalizeAddress 标准化地址格式（支持 Tron/ETH/BSC）
func NormalizeAddress(address, chain string) string {
	address = strings.TrimPrefix(address, "0x")
	address = strings.TrimPrefix(address, "41")
	address = strings.ToUpper(address)
	switch chain {
	case "tron":
		return "41" + address
	case "eth", "bsc":
		return "0x" + address
	default:
		return address
	}
}

// AdjustAmount 按精度调整金额
func AdjustAmount(amount *big.Int, decimals int) *big.Int {
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	return new(big.Int).Div(amount, factor)
}

// GetLogIndex 获取 logIndex
func GetLogIndex(logIndexMap map[string][]int32, txHash string) int32 {
	logIndexes := logIndexMap[txHash]
	if len(logIndexes) > 0 {
		return logIndexes[0]
	}
	return 0
}

// IsTargetContract 判断合约地址是否为目标合约
func IsTargetContract(contractAddress, targetAddress, chain string) bool {
	ca := NormalizeAddress(contractAddress, chain)
	ta := NormalizeAddress(targetAddress, chain)
	return ca == ta
}
