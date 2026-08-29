package captcha

import (
	"net"
	"os"
	"strconv"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/redis"
)

// setupRedis 建立真实 Redis 连接并返回客户端，未设置地址时跳过。
// 对照 pkg/redis 的集成测试约定：走 DB 15，不碰包级默认实例。
func setupRedis(t *testing.T) *goredis.Client {
	t.Helper()

	addr := os.Getenv("RUOYI_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("未设置 RUOYI_TEST_REDIS_ADDR，跳过真实 Redis 集成测试")
	}

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("RUOYI_TEST_REDIS_ADDR 格式应为 host:port, got %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("端口非法: %v", err)
	}

	client, err := redis.New(config.RedisConfig{
		Host:       host,
		Port:       port,
		Password:   os.Getenv("RUOYI_TEST_REDIS_PASSWORD"),
		DB:         15, // 测试专用库
		ClientName: "ruoyi-go-test",
		PoolSize:   8,
	})
	if err != nil {
		t.Fatalf("连接 Redis 失败: %v", err)
	}
	t.Cleanup(func() { _ = redis.Close(client) })
	return client
}

// newCaptcha 构造启用状态的验证码器，并注入测试用 Redis 客户端。
func newCaptcha(t *testing.T, cfg config.CaptchaConfig) *Captcha {
	t.Helper()

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	c.rdb = setupRedis(t)
	return c
}

// mathCaptcha 构造启用状态的算术验证码器。
func mathCaptcha(t *testing.T) *Captcha {
	t.Helper()

	return newCaptcha(t, config.CaptchaConfig{
		Enable: true, Type: config.CaptchaTypeMath, NumberLength: 1,
	})
}

// TestGenerateWritesAnswerToRedis 生成后 Redis 里存的应是**答案**而非题面，且带 2 分钟 TTL。
func TestGenerateWritesAnswerToRedis(t *testing.T) {
	c := mathCaptcha(t)
	ctx := t.Context()

	vo, err := c.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if !vo.CaptchaEnabled || vo.UUID == "" || vo.Img == "" {
		t.Fatalf("启用时应返回完整 Vo, got %+v", vo)
	}
	// 对照 Java IdUtil.simpleUUID()：32 位无连字符。
	if len(vo.UUID) != 32 {
		t.Errorf("uuid 应为 32 位无连字符, got %q", vo.UUID)
	}

	rdb := c.client()
	k := key(vo.UUID)
	t.Cleanup(func() { _ = rdb.Del(ctx, k).Err() })

	stored, err := rdb.Get(ctx, k).Result()
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", k, err)
	}
	// 存的必须是可解析的数字答案，而不是 "3+5=?" 这样的题面。
	if _, err := strconv.Atoi(stored); err != nil {
		t.Errorf("Redis 应存算术答案(纯数字), got %q", stored)
	}

	ttl, err := rdb.TTL(ctx, k).Result()
	if err != nil {
		t.Fatalf("读取 TTL 失败: %v", err)
	}
	if ttl <= 0 || ttl > expiration {
		t.Errorf("TTL 应在 (0, %v] 内, got %v", expiration, ttl)
	}
}

// TestValidateSuccess / WrongAnswer / Expired / CharIgnoreCase 已随校验逻辑迁至
// internal/auth/service/captcha_integration_test.go —— 校验(取值→删除→判空→比对)
// 现在在认证策略的 validateCaptcha 里，对照 Java PasswordAuthStrategy。
// 本包只保留生成侧用例。
