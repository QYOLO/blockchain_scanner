package utils

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// ParseTokenTransferInput 解析ERC20/TRC20的transfer input
func ParseTokenTransferInput(data string) (to string, amount *big.Int, err error) {
	data = strings.TrimPrefix(data, "0x")
	if len(data) < 136 {
		return "", nil, fmt.Errorf("invalid data length: got %d, want at least 136", len(data))
	}
	toParam := data[8+24 : 8+64]
	toBytes, err := hex.DecodeString(toParam)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode to address: %v, raw: %s", err, toParam)
	}
	if len(toBytes) > 20 {
		toBytes = toBytes[len(toBytes)-20:]
	}
	amount = new(big.Int)
	amountParam := data[72:136]
	if _, ok := amount.SetString(amountParam, 16); !ok {
		return "", nil, fmt.Errorf("failed to parse amount: %s", amountParam)
	}
	return strings.ToLower(fmt.Sprintf("0x%x", toBytes)), amount, nil
}
