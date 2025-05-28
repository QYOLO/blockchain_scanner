package main

import (
	"blockchain_scanner/internal/scanner"
	"blockchain_scanner/internal/storage"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// 初始化配置
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	// 初始化日志
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.Level.SetLevel(zap.DebugLevel)

	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	// 初始化数据库连接
	dbConfig := &storage.MySQLConfig{
		Host:      viper.GetString("mysql.host"),
		Port:      viper.GetInt("mysql.port"),
		Database:  viper.GetString("mysql.database"),
		Username:  viper.GetString("mysql.username"),
		Password:  viper.GetString("mysql.password"),
		Charset:   viper.GetString("mysql.charset"),
		ParseTime: viper.GetBool("mysql.parse_time"),
		Loc:       viper.GetString("mysql.loc"),
	}

	db, err := storage.NewMySQLConnection(dbConfig)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}

	// 获取链配置
	var chainConfig struct {
		Chain   string `mapstructure:"chain"`
		NodeURL string `mapstructure:"node_url"`
		Enabled bool   `mapstructure:"enabled"`
	}
	if err := viper.UnmarshalKey("chains.0", &chainConfig); err != nil {
		logger.Fatal("Failed to get chain config", zap.Error(err))
	}

	// 获取代币配置
	var tokenConfig struct {
		Chain           string `mapstructure:"chain"`
		ContractAddress string `mapstructure:"contract_address"`
		Symbol          string `mapstructure:"symbol"`
		Decimals        int    `mapstructure:"decimals"`
	}
	if err := viper.UnmarshalKey("tokens.0", &tokenConfig); err != nil {
		logger.Fatal("Failed to get token config", zap.Error(err))
	}

	// 初始化扫描器配置
	scannerConfig := &scanner.Config{
		StartBlock:        viper.GetInt64("scanner.start_block"),
		BatchSize:         viper.GetInt("scanner.batch_size"),
		ConcurrentWorkers: viper.GetInt("scanner.concurrent_workers"),
		RetryTimes:        viper.GetInt("scanner.retry_times"),
		RetryInterval:     viper.GetInt("scanner.retry_interval"),
		NodeURL:           chainConfig.NodeURL,
		Chain:             chainConfig.Chain,
	}

	// 初始化代币配置
	tokenConf := &scanner.TokenConfig{
		Chain:           tokenConfig.Chain,
		ContractAddress: tokenConfig.ContractAddress,
		Symbol:          tokenConfig.Symbol,
		Decimals:        tokenConfig.Decimals,
	}

	logger.Info("Starting with configuration",
		zap.Any("scanner_config", scannerConfig),
		zap.Any("token_config", tokenConf))

	// 创建扫描器实例
	blockScanner := scanner.NewBlockScanner(logger, db, scannerConfig, tokenConf)

	// 创建上下文和取消函数
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动扫描器
	if err := blockScanner.Start(ctx); err != nil {
		logger.Fatal("Failed to start scanner", zap.Error(err))
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// 优雅关闭
	logger.Info("Shutting down scanner...")
	blockScanner.Stop()
	logger.Info("Scanner stopped successfully")
}
