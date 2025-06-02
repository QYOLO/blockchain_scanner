package utils

import (
	"math/big"
	"strings"
)

// HexToInt64 将十六进制字符串转换为int64
func HexToInt64(hex string) int64 {
	hex = strings.TrimPrefix(hex, "0x")
	n := new(big.Int)
	n.SetString(hex, 16)
	return n.Int64()
}

// HexToUint64 将十六进制字符串转换为uint64
func HexToUint64(hex string) uint64 {
	hex = strings.TrimPrefix(hex, "0x")
	n := new(big.Int)
	n.SetString(hex, 16)
	return n.Uint64()
}
