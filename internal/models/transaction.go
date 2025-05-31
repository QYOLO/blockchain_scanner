package models

import (
	"time"
)

// Transaction 标准化交易数据
type Transaction struct {
	ID              uint      `gorm:"primaryKey"`
	Chain           string    `gorm:"type:varchar(20);not null;index"`
	ContractAddress string    `gorm:"type:varchar(66);not null;index"`
	TokenSymbol     string    `gorm:"type:varchar(10);not null"`
	Block           int64     `gorm:"not null;index"`
	LogIndex        int32     `gorm:"not null"`
	TxHash          string    `gorm:"type:varchar(66);not null;index"`
	Timestamp       time.Time `gorm:"not null;index"`
	FromAddress     string    `gorm:"type:varchar(66);not null;index"`
	ToAddress       string    `gorm:"type:varchar(66);not null;index"`
	Amount          int64     `gorm:"not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TableName 设置Transaction的表名
func (t *Transaction) TableName() string {
	return "transactions"
}

// DorisTransaction Doris写入数据结构
type DorisTransaction struct {
	ID                  uint      `gorm:"primaryKey"`
	OwnAddress          string    `gorm:"type:varchar(66);not null;uniqueIndex:idx_unique_tx"`
	Timestamp           time.Time `gorm:"not null;index"`
	CounterpartyAddress string    `gorm:"type:varchar(66);not null;index"`
	Amount              int64     `gorm:"not null"`
	Block               int64     `gorm:"not null;index"`
	LogIndex            int32     `gorm:"not null;uniqueIndex:idx_unique_tx"`
	TxHash              string    `gorm:"type:varchar(66);not null;uniqueIndex:idx_unique_tx"`
	Direction           int8      `gorm:"not null;uniqueIndex:idx_unique_tx"` // 0-进，1-出
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// TableName 设置DorisTransaction的表名
func (dt *DorisTransaction) TableName() string {
	return "doris_transactions"
}
