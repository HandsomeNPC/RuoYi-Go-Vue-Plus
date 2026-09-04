package ratelimiter

import (
	"net"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/middleware"
	"ruoyi-go-vue-plus/pkg/redis"
)

// setupRedis 建立真实 Redis 客户端，未设置地址时跳过。对照 pkg/redis 的集成测试约定。
func setupRedis(t *testing.T) *Limiter {
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

	return &Limiter{instanceID: "test-inst-1", rdb: client}
}

// randKeySuffix 生成测试专用的唯一 key 后缀，避免测试间相互影响。
func randKeySuffix() string { return newMemberSuffix() }

// TestAcquireEnforcesLimit 窗口内第 count+1 次必须被拒。
func TestAcquireEnforcesLimit(t *testing.T) {
	l := setupRedis(t)
	ctx := t.Context()

	// count=3, 窗口 1min。用 1min 保证测试期间窗口不滑过。
	o := &options{window: time.Minute, count: 3, timeout: 2 * time.Minute}
	key := "global:rate_limit:test:unit:" + randKeySuffix()

	for i := 2; i >= 0; i-- {
		remain, err := l.acquire(ctx, key, o)
		if err != nil {
			t.Fatalf("第 %d 次 acquire: %v", 3-i, err)
		}
		if remain != int64(i) {
			t.Errorf("第 %d 次剩余 = %d, want %d", 3-i, remain, i)
		}
	}
	if remain, err := l.acquire(ctx, key, o); err != nil {
		t.Fatalf("超限 acquire: %v", err)
	} else if remain != -1 {
		t.Errorf("超限应返回 -1, got %d", remain)
	}
}

// TestAcquireDifferentKeysIndependent 不同 key 互不影响（模拟不同 IP/路径各自计额度）。
func TestAcquireDifferentKeysIndependent(t *testing.T) {
	l := setupRedis(t)
	ctx := t.Context()

	o := &options{window: time.Minute, count: 1, timeout: 2 * time.Minute}
	k1 := "global:rate_limit:test:ind:" + randKeySuffix() + ":a"
	k2 := "global:rate_limit:test:ind:" + randKeySuffix() + ":b"

	if _, err := l.acquire(ctx, k1, o); err != nil {
		t.Fatalf("k1 第 1 次: %v", err)
	}
	if remain, _ := l.acquire(ctx, k1, o); remain != -1 {
		t.Errorf("k1 第 2 次应被拒, got remain=%d", remain)
	}
	if remain, err := l.acquire(ctx, k2, o); err != nil || remain == -1 {
		t.Errorf("k2 第 1 次应成功, remain=%d err=%v", remain, err)
	}
}

// TestHandlerRendersRejection 触发限流时走完整 gin 链路应返回 200 + code 500 + 提示文案。
func TestHandlerRendersRejection(t *testing.T) {
	l := setupRedis(t)

	// count=1：首次放行，第二次命中限流。
	r := gin.New()
	r.Use(middleware.Recover())
	r.GET("/auth/code", l.middleware(
		nil, // 动态维度
		time.Minute, 1, LimitTypeIP, 0, "",
	), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	doGet := func() (int, string) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/auth/code", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		r.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	code, body := doGet()
	if code != 200 || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("第 1 次应放行, code=%d body=%q", code, body)
	}

	code, body = doGet()
	if code != 200 {
		t.Errorf("限流 HTTP 码 = %d, want 200 (对齐 Java 非 429)", code)
	}
	if !strings.Contains(body, "访问过于频繁") {
		t.Errorf("限流文案应为中文, got %q", body)
	}
	if !strings.Contains(body, "code") {
		t.Errorf("响应应含 code 字段: %q", body)
	}
}

// middleware 包内测试用：把 run 包装成 gin.HandlerFunc。
func (l *Limiter) middleware(fn func(*gin.Context) string, window time.Duration, count int, limitType LimitType, timeout time.Duration, message string) gin.HandlerFunc {
	o := newOptions(window, count, limitType, timeout, message)
	if fn != nil {
		o.keyFunc = fn
	}
	return func(c *gin.Context) { l.run(c, o) }
}

// TestRedisDownFailsOpen Redis 不可用时应放行而非阻断业务（可用性优先）。
func TestRedisDownFailsOpen(t *testing.T) {
	// 指向一个必然连不上的端口，不需要真实 Redis，故不调 setupRedis。
	bad := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = bad.Close() })
	l := &Limiter{instanceID: "down", rdb: bad}

	r := gin.New()
	r.Use(middleware.Recover())
	r.GET("/t", l.middleware(nil, 0, 1, 0, 0, ""), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// count=1，但 Redis 挂了，三次都应放行。
	for i := 1; i <= 3; i++ {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", "/t", nil))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"ok":true`) {
			t.Errorf("第 %d 次: Redis 挂时应放行, code=%d body=%q",
				i, rec.Code, rec.Body.String())
		}
	}
}
