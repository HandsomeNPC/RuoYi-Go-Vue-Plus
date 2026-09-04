// Package cache 通用 Redis 缓存助手。
//
// 键格式：group + ":" + key，与 pkg/constant/cache_names.go 的组名和 TTL 常量配合使用。
// 所有结构体序列化走 pkg/jsonx，保持与 API 出参相同的 int64 精度契约。
// Redis 操作均 fail-open：调用方收到 false/空错误时继续走 DB 或忽略即可。
package cache

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/redis"
)

var sf singleflight.Group

// cacheKey 组合最终 Redis key：group:key。
func cacheKey(group, key string) string {
	return group + ":" + key
}

func rdb() *goredis.Client {
	return redis.Client()
}

// Get 尝试从缓存读取并反序列化到 dest。
// 命中返回 (true, nil)；miss 返回 (false, nil)；Redis 故障返回 (false, err)。
func Get(ctx context.Context, group, key string, dest any) (bool, error) {
	data, err := rdb().Get(ctx, cacheKey(group, key)).Bytes()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, jsonx.Unmarshal(data, dest)
}

// Put 将 val 序列化后写入缓存。ttl=0 表示不过期。
func Put(ctx context.Context, group, key string, val any, ttl time.Duration) error {
	data, err := jsonx.Marshal(val)
	if err != nil {
		return err
	}
	return rdb().Set(ctx, cacheKey(group, key), data, ttl).Err()
}

// Evict 删除单个缓存条目。key 不存在时幂等。
func Evict(ctx context.Context, group, key string) error {
	return rdb().Del(ctx, cacheKey(group, key)).Err()
}

// EvictGroup 删除整个缓存组的所有条目。
// 用 SCAN 分批枚举 group:* 后 pipeline Del，避免 KEYS 指令阻塞 Redis。
func EvictGroup(ctx context.Context, group string) error {
	pattern := group + ":*"
	r := rdb()
	var cursor uint64
	for {
		keys, next, err := r.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

// GetOrSet 读穿缓存：命中直接反序列化到 dest 返回，miss 时调 load 查源并写缓存。
// 用 singleflight 合并对同一 key 的并发 miss，防缓存击穿。
// load 出错时不写缓存，错误透传。
func GetOrSet(ctx context.Context, group, key string, ttl time.Duration,
	dest any, load func(context.Context) (any, error)) error {

	k := cacheKey(group, key)
	r := rdb()

	if data, err := r.Get(ctx, k).Bytes(); err == nil {
		return jsonx.Unmarshal(data, dest)
	} else if !errors.Is(err, goredis.Nil) {
		return err
	}

	// miss：singleflight 合并同 key 并发请求。
	raw, err, _ := sf.Do(k, func() (any, error) {
		// 二次检查：等待期间可能已被其它协程写入。
		if data, err2 := r.Get(ctx, k).Bytes(); err2 == nil {
			return data, nil
		}
		v, err2 := load(ctx)
		if err2 != nil {
			return nil, err2
		}
		b, err2 := jsonx.Marshal(v)
		if err2 != nil {
			return nil, err2
		}
		_ = r.Set(ctx, k, b, ttl).Err()
		return b, nil
	})
	if err != nil {
		return err
	}
	return jsonx.Unmarshal(raw.([]byte), dest)
}
