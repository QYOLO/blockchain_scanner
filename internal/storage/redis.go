package storage

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-redis/redis/v8"
)

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

type ProgressStore struct {
	client *redis.Client
}

func NewRedisConnection(config *RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %v", err)
	}

	return client, nil
}

func NewProgressStore(client *redis.Client) *ProgressStore {
	return &ProgressStore{
		client: client,
	}
}

// SaveProgress 保存扫描进度
func (s *ProgressStore) SaveProgress(ctx context.Context, chain string, block int64) error {
	key := fmt.Sprintf("scanner:%s:progress", chain)
	return s.client.Set(ctx, key, block, 0).Err()
}

// GetProgress 获取扫描进度
func (s *ProgressStore) GetProgress(ctx context.Context, chain string) (int64, error) {
	key := fmt.Sprintf("scanner:%s:progress", chain)
	val, err := s.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}
