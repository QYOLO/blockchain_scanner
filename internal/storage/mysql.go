package storage

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type MySQLConfig struct {
	Host      string
	Port      int
	Database  string
	Username  string
	Password  string
	Charset   string
	ParseTime bool
	Loc       string
}

func NewMySQLConnection(config *MySQLConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%v&loc=%s",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		config.Charset,
		config.ParseTime,
		config.Loc,
	)

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %v", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 初始化数据库表
	if err := InitDB(db); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %v", err)
	}

	return db, nil
}

// InitDB 初始化数据库表
func InitDB(db *gorm.DB) error {
	// 创建交易表
	err := db.AutoMigrate(&Transaction{})
	if err != nil {
		return fmt.Errorf("failed to create transactions table: %v", err)
	}

	// 创建TRON USDT双向记录表
	err = db.AutoMigrate(&TronUsdtTransaction{})
	if err != nil {
		return fmt.Errorf("failed to create tron_usdt_transactions table: %v", err)
	}

	return nil
}
