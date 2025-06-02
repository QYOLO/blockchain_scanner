package utils

import (
	"encoding/hex"
	"strings"
)

// ConvertToTronAddress 将以太坊格式地址转换为TRON格式
func ConvertToTronAddress(ethAddress string) string {
	addr := strings.TrimPrefix(ethAddress, "0x")
	return "41" + addr
}

// ConvertToHexAddress 将TRON格式地址转换为以太坊格式
func ConvertToHexAddress(tronAddress string) string {
	addr := strings.TrimPrefix(tronAddress, "41")
	return "0x" + addr
}

// IsValidTronAddress 检查是否是有效的TRON地址
func IsValidTronAddress(address string) bool {
	address = strings.TrimPrefix(address, "0x")
	if !strings.HasPrefix(address, "41") {
		return false
	}
	address = strings.TrimPrefix(address, "41")
	if len(address) != 40 {
		return false
	}
	_, err := hex.DecodeString(address)
	return err == nil
}
