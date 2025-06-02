package scanner

import "blockchain_scanner/internal/storage"

// 脱敏工具
func maskString(s string, head, tail int) string {
	if len(s) <= head+tail {
		return s
	}
	return s[:head] + "****" + s[len(s)-tail:]
}

func maskTransactions(txs []*storage.Transaction, n int) []map[string]interface{} {
	var result []map[string]interface{}
	for i, tx := range txs {
		if i >= n {
			break
		}
		result = append(result, map[string]interface{}{
			"from":   maskString(tx.FromAddress, 4, 4),
			"to":     maskString(tx.ToAddress, 4, 4),
			"hash":   maskString(tx.TxHash, 4, 4),
			"block":  tx.Block,
			"amount": tx.Amount,
		})
	}
	return result
}
