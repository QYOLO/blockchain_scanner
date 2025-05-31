package main

import (
	"blockchain_scanner/internal/scanner"
	"blockchain_scanner/internal/storage"
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func main() {
	initConfig()
	logger := initLogger()
	defer logger.Sync()

	db := mustInitDB(logger)
	scannerConfig, tokenConfig := loadChainAndTokenConfig(logger)

	blockScanner := scanner.NewBlockScanner(logger, db, scannerConfig, tokenConfig)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := blockScanner.Start(ctx); err != nil {
		logger.Fatal("Failed to start scanner", zap.Error(err))
	}

	waitForShutdown(logger, blockScanner)
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config")
	if err := viper.ReadInConfig(); err != nil {
		panic("Error reading config file: " + err.Error())
	}
}

func initLogger() *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	return logger
}

func mustInitDB(logger *zap.Logger) *gorm.DB {
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
	return db
}

func loadChainAndTokenConfig(logger *zap.Logger) (*scanner.Config, *scanner.TokenConfig) {
	var chainConfig struct {
		Chain   string `mapstructure:"chain"`
		NodeURL string `mapstructure:"node_url"`
	}
	if err := viper.UnmarshalKey("chains.0", &chainConfig); err != nil {
		logger.Fatal("Failed to get chain config", zap.Error(err))
	}
	var tokenConfig struct {
		Chain           string `mapstructure:"chain"`
		ContractAddress string `mapstructure:"contract_address"`
		Symbol          string `mapstructure:"symbol"`
		Decimals        int    `mapstructure:"decimals"`
	}
	if err := viper.UnmarshalKey("tokens.0", &tokenConfig); err != nil {
		logger.Fatal("Failed to get token config", zap.Error(err))
	}
	return &scanner.Config{
			StartBlock:        viper.GetInt64("scanner.start_block"),
			BatchSize:         viper.GetInt("scanner.batch_size"),
			ConcurrentWorkers: viper.GetInt("scanner.concurrent_workers"),
			RetryTimes:        viper.GetInt("scanner.retry_times"),
			RetryInterval:     viper.GetInt("scanner.retry_interval"),
			NodeURL:           chainConfig.NodeURL,
			Chain:             chainConfig.Chain,
		}, &scanner.TokenConfig{
			Chain:           tokenConfig.Chain,
			ContractAddress: tokenConfig.ContractAddress,
			Symbol:          tokenConfig.Symbol,
			Decimals:        tokenConfig.Decimals,
		}
}

func waitForShutdown(logger *zap.Logger, blockScanner *scanner.BlockScanner) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	logger.Info("Shutting down scanner...")
	blockScanner.Stop()
	logger.Info("Scanner stopped successfully")
}
