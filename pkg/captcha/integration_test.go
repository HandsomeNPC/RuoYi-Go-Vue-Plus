package captcha

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/errs"
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

// TestValidateSuccess 答案正确应通过，且校验后键被删除(一次性)。
func TestValidateSuccess(t *testing.T) {
	c := mathCaptcha(t)
	ctx := t.Context()

	vo, err := c.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	rdb := c.client()
	k := key(vo.UUID)
	t.Cleanup(func() { _ = rdb.Del(ctx, k).Err() })

	answer, err := rdb.Get(ctx, k).Result()
	if err != nil {
		t.Fatalf("读取答案失败: %v", err)
	}

	if err := c.Validate(ctx, vo.UUID, answer); err != nil {
		t.Fatalf("正确答案应校验通过, got %v", err)
	}

	// 校验后键必须已删除。
	if err := rdb.Get(ctx, k).Err(); !errors.Is(err, goredis.Nil) {
		t.Errorf("校验后键应被删除, got err=%v", err)
	}
	// 同一 uuid 二次校验必须失败，杜绝重放。
	if err := c.Validate(ctx, vo.UUID, answer); err == nil {
		t.Error("同一 uuid 二次校验应失败")
	}
}

// TestValidateWrongAnswerConsumesCode 答案错误时也必须删键，杜绝同一 uuid 反复试错。
func TestValidateWrongAnswerConsumesCode(t *testing.T) {
	c := mathCaptcha(t)
	ctx := t.Context()

	vo, err := c.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	rdb := c.client()
	k := key(vo.UUID)
	t.Cleanup(func() { _ = rdb.Del(ctx, k).Err() })

	err = c.Validate(ctx, vo.UUID, "绝不可能是答案")
	if err == nil {
		t.Fatal("错误答案应校验失败")
	}
	var se *errs.ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("应返回 ServiceError, got %T", err)
	}
	if se.Msg != "验证码错误" {
		t.Errorf("提示应为 验证码错误, got %q", se.Msg)
	}

	if err := rdb.Get(ctx, k).Err(); !errors.Is(err, goredis.Nil) {
		t.Errorf("答错后键也应被删除, got err=%v", err)
	}
}

// TestValidateExpired uuid 不存在(过期/伪造)应返回"已失效"。
func TestValidateExpired(t *testing.T) {
	c := mathCaptcha(t)

	err := c.Validate(t.Context(), "根本不存在的uuid", "1")
	if err == nil {
		t.Fatal("不存在的 uuid 应校验失败")
	}
	var se *errs.ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("应返回 ServiceError, got %T", err)
	}
	if se.Msg != "验证码已失效" {
		t.Errorf("提示应为 验证码已失效, got %q", se.Msg)
	}
}

// TestValidateCharIgnoreCase 字符验证码应忽略大小写，对照 Java equalsIgnoreCase。
func TestValidateCharIgnoreCase(t *testing.T) {
	ctx := t.Context()

	c := newCaptcha(t, config.CaptchaConfig{
		Enable: true, Type: config.CaptchaTypeChar, CharLength: 4,
	})

	vo, err := c.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	rdb := c.client()
	k := key(vo.UUID)
	t.Cleanup(func() { _ = rdb.Del(ctx, k).Err() })

	answer, err := rdb.Get(ctx, k).Result()
	if err != nil {
		t.Fatalf("读取答案失败: %v", err)
	}

	// 全部翻成大写后仍应通过。
	if err := c.Validate(ctx, vo.UUID, strings.ToUpper(answer)); err != nil {
		t.Errorf("大小写不同应仍校验通过, got %v", err)
	}
}
