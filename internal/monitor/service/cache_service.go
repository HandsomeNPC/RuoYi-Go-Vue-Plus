// Package service 缓存监控业务逻辑。
package service

import (
	"context"
	"strings"

	"ruoyi-go-vue-plus/internal/monitor/model/vo"
	"ruoyi-go-vue-plus/pkg/errs"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/response"
)

// CacheSvc 缓存监控服务。
type CacheSvc struct{}

var CacheSvcApp = new(CacheSvc)

// GetInfo 取 Redis 服务信息、库大小与命令统计饼图数据。
//
// 直接用包级 Redis 客户端而非走 repository：本仓库只有 repository 层碰 GORM，
// Redis 不是 GORM，user_service 等既有代码也是 service 直连 redis.Client()。
func (s *CacheSvc) GetInfo(ctx context.Context) (*vo.CacheListInfoVo, error) {
	client := pkgredis.Client()

	infoStr, err := client.Info(ctx).Result()
	if err != nil {
		return nil, errs.New(response.CodeFail, "获取 Redis 信息失败", err.Error())
	}

	dbSize, err := client.DBSize(ctx).Result()
	if err != nil {
		return nil, errs.New(response.CodeFail, "获取 Redis 库大小失败", err.Error())
	}

	cmdStatsStr, err := client.Info(ctx, "commandstats").Result()
	if err != nil {
		return nil, errs.New(response.CodeFail, "获取 Redis 命令统计失败", err.Error())
	}

	return &vo.CacheListInfoVo{
		Info:         parseInfo(infoStr),
		DBSize:       dbSize,
		CommandStats: parseCommandStats(cmdStatsStr),
	}, nil
}

// parseInfo 解析 Redis INFO 文本为键值映射，注释行(# 开头)与空行跳过。
func parseInfo(s string) map[string]string {
	m := make(map[string]string)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, ':'); i >= 0 {
			m[line[:i]] = line[i+1:]
		}
	}
	return m
}

// parseCommandStats 解析 commandstats 段为饼图数据：每条形如
// cmdstat_get:calls=37,usec=12,... 取 name=get、value=37（calls 数）。
func parseCommandStats(s string) []map[string]string {
	var list []map[string]string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "cmdstat_") {
			continue
		}
		rest := strings.TrimPrefix(line, "cmdstat_")
		colon := strings.IndexByte(rest, ':')
		if colon < 0 {
			continue
		}
		name := rest[:colon]
		calls := substringBetween(rest[colon+1:], "calls=", ",usec")
		if calls == "" {
			continue
		}
		list = append(list, map[string]string{"name": name, "value": calls})
	}
	return list
}

// substringBetween 取 start 与 end 之间首段子串，任一不存在返回空串。
func substringBetween(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	i += len(start)
	j := strings.Index(s[i:], end)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}
