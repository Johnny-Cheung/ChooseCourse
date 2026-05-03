package repository

import (
	"context"
	"fmt"

	appconfig "choose-course-backend/internal/config"
	"github.com/redis/go-redis/v9"
)

// redisClient 保存全局 Redis 客户端实例。
var redisClient *redis.Client

// InitRedis 初始化 Redis 客户端，并在启动时做连通性检查。
func InitRedis(cfg appconfig.RedisConfig) error {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		return err
	}

	redisClient = client
	return nil
}

// Redis 返回全局 Redis 客户端。
func Redis() *redis.Client {
	return redisClient
}

// PingRedis 用于健康检查接口。
func PingRedis(ctx context.Context) error {
	if redisClient == nil {
		return fmt.Errorf("redis not initialized")
	}

	return redisClient.Ping(ctx).Err()
}

// CloseRedis 在服务退出时关闭 Redis 连接。
func CloseRedis() {
	if redisClient == nil {
		return
	}

	_ = redisClient.Close()
}
