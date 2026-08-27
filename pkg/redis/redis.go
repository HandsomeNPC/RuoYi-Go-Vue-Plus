// Package redis Redis 客户端初始化。
package redis

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/config"
)

// pingTimeout 启动时探活 Redis 的超时时间。
const pingTimeout = 5 * time.Second

// New 按配置建立 Redis 客户端并完成连接池设置。
func New(cfg config.RedisConfig) (*goredis.Client, error) {
	client := goredis.NewClient(&goredis.Options{
		Addr:            cfg.Addr(),
		Password:        cfg.Password,
		DB:              cfg.DB,
		ClientName:      cfg.ClientName,
		PoolSize:        cfg.PoolSize,
		MinIdleConns:    cfg.MinIdleConns,
		DialTimeout:     cfg.DialTimeout(),
		ReadTimeout:     cfg.ReadTimeout(),
		WriteTimeout:    cfg.WriteTimeout(),
		ConnMaxIdleTime: cfg.MaxIdleTime(),
	})

	if err := ping(client); err != nil {
		// 探活失败也要回收连接池。
		_ = client.Close()
		return nil, fmt.Errorf("redis: 连接 %s db=%d 失败: %w", cfg.Addr(), cfg.DB, err)
	}
	return client, nil
}

// ping 启动探活，确认 Redis 真实可用。
func ping(client *goredis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("探活失败: %w", err)
	}
	return nil
}

// Close 关闭客户端，供进程退出时调用。
func Close(client *goredis.Client) error {
	if client == nil {
		return nil
	}
	return client.Close()
}

// 包级默认实例。
var (
	mu            sync.RWMutex
	defaultClient *goredis.Client
)

// Init 建立连接并设置为包级默认实例。
func Init() {
	c := config.Get()
	cfg := c.Redis
	client, err := New(cfg)
	if err != nil {
		panic(fmt.Errorf("redis: 初始化失败: %w", err))
	}
	mu.Lock()
	defaultClient = client
	mu.Unlock()
	log.Printf("[%s] Redis 已连接 %s db=%d", c.Server.Name, cfg.Addr(), cfg.DB)
}

// Client 返回包级默认实例，未调用 Init 会 panic。
func Client() *goredis.Client {
	mu.RLock()
	client := defaultClient
	mu.RUnlock()
	if client == nil {
		panic("redis: 尚未初始化，请先调用 redis.Init")
	}
	return client
}

// CloseDefault 关闭并清空包级默认实例，供进程退出时 defer 调用。
func CloseDefault() {
	mu.Lock()
	client := defaultClient
	defaultClient = nil
	mu.Unlock()
	if err := Close(client); err != nil {
		log.Printf("关闭 Redis 连接失败: %v", err)
	}
}
