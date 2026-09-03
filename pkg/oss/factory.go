package oss

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
)

// ErrConfigNotFound 配置键对应的配置不在缓存里。
// 多半是没跑配置预热，或该 configKey 已被删除。
var ErrConfigNotFound = errors.New("oss: 配置信息不存在")

// ErrDefaultConfigNotSet 没有任何配置被标记为默认。
var ErrDefaultConfigNotSet = errors.New("oss: 文件存储服务类型无法找到")

// 客户端按 configKey 缓存。建客户端要建连接池，每次上传都重建太浪费。
var (
	clientsMu sync.Mutex
	clients   = make(map[string]*Client)
)

// InstanceDefault 取当前默认配置的客户端，供上传用。
func InstanceDefault(ctx context.Context) (*Client, error) {
	configKey, err := pkgredis.Client().Get(ctx, constant.OssDefaultConfigKey).Result()
	if err != nil || configKey == "" {
		return nil, ErrDefaultConfigNotSet
	}
	return Instance(ctx, configKey)
}

// Instance 取指定配置键的客户端，供下载/删除按文件记录的 service 列取用。
//
// 配置变了不需要显式失效：每次都重新解析缓存里的配置，与已缓存客户端的配置比对，
// 不一致就重建（对齐 Java OssFactory 的 verifyConfig）。Remove 只是主动清理。
func Instance(ctx context.Context, configKey string) (*Client, error) {
	if configKey == "" {
		return nil, ErrDefaultConfigNotSet
	}

	var props Properties
	hit, err := cache.Get(ctx, constant.CacheSysOssConfig, configKey, &props)
	if err != nil {
		return nil, fmt.Errorf("oss: 读取配置 %q 失败: %w", configKey, err)
	}
	if !hit {
		return nil, fmt.Errorf("%w: %s", ErrConfigNotFound, configKey)
	}
	cfg := NewClientConfig(props)

	clientsMu.Lock()
	defer clientsMu.Unlock()

	if c, ok := clients[configKey]; ok && c.cfg == cfg {
		return c, nil
	}

	c, err := newClient(configKey, cfg)
	if err != nil {
		return nil, err
	}
	clients[configKey] = c
	return c, nil
}

// Remove 丢弃指定配置键的客户端缓存，供配置变更/删除后主动清理。
func Remove(configKey string) {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	delete(clients, configKey)
}
