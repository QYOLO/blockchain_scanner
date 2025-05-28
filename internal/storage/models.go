package storage

import (
	"time"

	"gorm.io/gorm"
)

// Transaction 原始交易记录
type Transaction struct {
	ID              uint64         `gorm:"primaryKey;autoIncrement"`
	Chain           string         `gorm:"type:varchar(20);not null;column:chain"`
	ContractAddress string         `gorm:"type:varchar(66);not null;column:contract_address"`
	TokenSymbol     string         `gorm:"type:varchar(10);not null;column:token_symbol"`
	Block           int64          `gorm:"not null;index;column:block"`
	LogIndex        int32          `gorm:"not null;column:log_index"`
	TxHash          string         `gorm:"type:varchar(66);uniqueIndex;not null;column:tx_hash"`
	Timestamp       time.Time      `gorm:"index;not null;column:timestamp"`
	FromAddress     string         `gorm:"type:varchar(42);index;not null;column:from_address"`
	ToAddress       string         `gorm:"type:varchar(42);index;not null;column:to_address"`
	Amount          int64          `gorm:"not null;column:amount"`
	CreatedAt       time.Time      `gorm:"not null"`
	UpdatedAt       time.Time      `gorm:"not null"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

// TronUsdtTransaction TRON USDT 双向记录
type TronUsdtTransaction struct {
	ID                  uint64         `gorm:"primaryKey;autoIncrement"`
	OwnAddress          string         `gorm:"type:varchar(42);index;not null;column:own_address"`
	Timestamp           time.Time      `gorm:"index;not null;column:timestamp"`
	CounterpartyAddress string         `gorm:"type:varchar(42);index;not null;column:counterparty_address"`
	Amount              int64          `gorm:"not null;column:amount"`
	Block               int64          `gorm:"not null;index;column:block"`
	LogIndex            int32          `gorm:"not null;column:log_index"`
	TxHash              string         `gorm:"type:varchar(66);index;not null;column:tx_hash"`
	Direction           int8           `gorm:"not null;column:direction"` // 0-进，1-出
	CreatedAt           time.Time      `gorm:"not null"`
	UpdatedAt           time.Time      `gorm:"not null"`
	DeletedAt           gorm.DeletedAt `gorm:"index"`
}

// TableName 设置Transaction表名
func (Transaction) TableName() string {
	return "transactions"
}

// TableName 设置TronUsdtTransaction表名
func (TronUsdtTransaction) TableName() string {
	return "tron_usdt_transactions"
}
