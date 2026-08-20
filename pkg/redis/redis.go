// Package redis Redis 客户端初始化(缓存 / 会话 / 分布式锁)。
//
// 两种用法，与 pkg/database 对称，按需选择：
//
//	rdb, err := redis.New(cfg.Redis)   // 返回实例，自行注入
//	err := redis.Init(cfg.Redis)       // 同时设置为包级默认，redis.Client() 取用
//
// 对应原项目的 Redisson 单节点配置(redisson.singleServerConfig)。
// 会话/缓存的 key 前缀等业务约定在 pkg/constant 定义，本包只管连接。
package redis

import (
	"context"
	"fmt"
	"sync"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/config"
)

// pingTimeout 启动时探活 Redis 的超时时间。
const pingTimeout = 5 * time.Second

// New 按配置建立 Redis 客户端并完成连接池设置。
//
// 返回前会 Ping 一次，连不上直接报错，避免进程带着坏连接启动；
// 失败时关闭已建立的连接，不泄漏。
func New(cfg config.Redis) (*goredis.Client, error) {
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
		// 探活失败也要回收连接池，否则后台 goroutine 与连接会残留。
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

// 包级默认实例。供 Init / Client / CloseDefault 使用，读写加锁以免竞态。
var (
	mu            sync.RWMutex
	defaultClient *goredis.Client
)

// Init 建立连接并设置为包级默认实例。
func Init(cfg config.Redis) error {
	client, err := New(cfg)
	if err != nil {
		return err
	}
	mu.Lock()
	defaultClient = client
	mu.Unlock()
	return nil
}

// Client 返回包级默认实例。未调用 Init 会 panic——
// 这是启动期编排错误，不该留到运行时才发现。
func Client() *goredis.Client {
	mu.RLock()
	client := defaultClient
	mu.RUnlock()
	if client == nil {
		panic("redis: 尚未初始化，请先调用 redis.Init")
	}
	return client
}

// CloseDefault 关闭并清空包级默认实例。
func CloseDefault() error {
	mu.Lock()
	client := defaultClient
	defaultClient = nil
	mu.Unlock()
	return Close(client)
}
