package service

import (
	"errors"
	"net"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/captcha"
	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/redis"
)

// 本文件覆盖 passwordAuthStrategy.validateCaptcha 的 Redis 行为（一次性消费、
// 答错也删键、忽略大小写、失效提示）。这些用例原在 pkg/captcha/integration_test.go，
// 随校验逻辑一并迁来——对照 Java，校验属于认证策略而非验证码组件。
//
// 注意：失败分支会异步记登录失败日志，其中落库需要 database.Init()。本测试不初始化
// 数据库，那次落库会 panic 并被 RecordLoginInfo 内的 recover 兜住（只打日志），
// 不影响断言；日志里出现「记录登录日志 panic」属预期。

// setupCaptchaRedis 建立真实 Redis 连接并设为包级默认实例，未设置地址时跳过。
// 对照 pkg/redis 的集成测试约定：走 DB 15。
func setupCaptchaRedis(t *testing.T) *goredis.Client {
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
	// validateCaptcha 走包级 redis.Client()，须注入后再调。
	t.Cleanup(redis.SetClient(client))
	t.Cleanup(func() { _ = redis.Close(client) })
	return client
}

// seedCaptcha 直接在 Redis 里种一个验证码答案，返回 uuid。
// 不走 captcha.Generate，免得测试依赖验证码开关与绘图。
func seedCaptcha(t *testing.T, rdb *goredis.Client, answer string) string {
	t.Helper()

	const uuid = "test-uuid-validate-captcha"
	k := constant.CaptchaCodeKey + uuid
	if err := rdb.Set(t.Context(), k, answer, 0).Err(); err != nil {
		t.Fatalf("写入验证码失败: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Del(t.Context(), k).Err() })
	return uuid
}

// validateCaptcha 以最小请求调被测方法。
func callValidateCaptcha(t *testing.T, code, uuid string) error {
	t.Helper()

	req := httptest.NewRequest("POST", "/login", nil)
	return new(passwordAuthStrategy).validateCaptcha(req, "testuser", code, uuid)
}

// TestValidateCaptchaSuccess 答案正确应通过，且校验后键被删除(一次性)。
func TestValidateCaptchaSuccess(t *testing.T) {
	rdb := setupCaptchaRedis(t)
	uuid := seedCaptcha(t, rdb, "8")

	if err := callValidateCaptcha(t, "8", uuid); err != nil {
		t.Fatalf("正确答案应校验通过, got %v", err)
	}

	// 校验后键必须已删除。
	k := constant.CaptchaCodeKey + uuid
	if err := rdb.Get(t.Context(), k).Err(); !errors.Is(err, goredis.Nil) {
		t.Errorf("校验后键应被删除, got err=%v", err)
	}
	// 同一 uuid 二次校验必须失败，杜绝重放。
	if err := callValidateCaptcha(t, "8", uuid); err == nil {
		t.Error("同一 uuid 二次校验应失败")
	}
}

// TestValidateCaptchaWrongAnswerConsumesCode 答案错误时也必须删键，杜绝同一 uuid 反复试错。
func TestValidateCaptchaWrongAnswerConsumesCode(t *testing.T) {
	rdb := setupCaptchaRedis(t)
	uuid := seedCaptcha(t, rdb, "8")

	err := callValidateCaptcha(t, "绝不可能是答案", uuid)
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

	k := constant.CaptchaCodeKey + uuid
	if err := rdb.Get(t.Context(), k).Err(); !errors.Is(err, goredis.Nil) {
		t.Errorf("答错后键也应被删除, got err=%v", err)
	}
}

// TestValidateCaptchaExpired uuid 不存在(过期/伪造)应返回"已失效"。
func TestValidateCaptchaExpired(t *testing.T) {
	setupCaptchaRedis(t)

	err := callValidateCaptcha(t, "1", "根本不存在的uuid")
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

// TestValidateCaptchaIgnoreCase 字符验证码应忽略大小写，对照 Java equalsIgnoreCase。
func TestValidateCaptchaIgnoreCase(t *testing.T) {
	rdb := setupCaptchaRedis(t)
	const answer = "aBcD"
	uuid := seedCaptcha(t, rdb, answer)

	// 全部翻成大写后仍应通过。
	if err := callValidateCaptcha(t, strings.ToUpper(answer), uuid); err != nil {
		t.Errorf("大小写不同应仍校验通过, got %v", err)
	}
}

// TestCaptchaGenerateStillWritesSameKey pkg/captcha 写入的键必须与本层读取的键一致，
// 否则生成的验证码永远校验不过。两侧共用 constant.CaptchaCodeKey，此处做回归防护。
func TestCaptchaGenerateStillWritesSameKey(t *testing.T) {
	rdb := setupCaptchaRedis(t)

	c, err := captcha.New(config.CaptchaConfig{
		Enable: true, Type: config.CaptchaTypeMath, NumberLength: 1,
	})
	if err != nil {
		t.Fatalf("captcha.New 失败: %v", err)
	}
	vo, err := c.Generate(t.Context())
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	k := constant.CaptchaCodeKey + vo.UUID
	t.Cleanup(func() { _ = rdb.Del(t.Context(), k).Err() })

	answer, err := rdb.Get(t.Context(), k).Result()
	if err != nil {
		t.Fatalf("按本层约定的键 %s 读不到 Generate 写入的答案: %v", k, err)
	}
	// 用读到的答案走完整校验链路，确认生成与校验闭合。
	if err := callValidateCaptcha(t, answer, vo.UUID); err != nil {
		t.Errorf("Generate 出的验证码应能通过 validateCaptcha, got %v", err)
	}
}
