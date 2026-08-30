package repeatsubmit

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

// setupRedis 建立真实 Redis 客户端，未设置地址时跳过。对照 pkg/ratelimiter 的集成测试约定。
func setupRedis(t *testing.T) *Submitter {
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

	return &Submitter{tokenName: "Authorization", rdb: client}
}

// middleware 把实例方法包成中间件，供集成测试绕开包级实例。
func (s *Submitter) middleware(opts ...Option) gin.HandlerFunc {
	o := newOptions(opts...)
	return func(c *gin.Context) { s.run(c, o) }
}

// newEngine 装配一条与生产一致的链路：Recover + RepeatableBody + 防重注解。
func newEngine(s *Submitter, handler gin.HandlerFunc, opts ...Option) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Recover())
	r.Use(middleware.RepeatableBodyWithConfig(config.DefaultRepeatableBody()))
	r.POST("/system/user", s.middleware(opts...), handler)
	return r
}

// post 发一次带 token 的 JSON POST，返回状态码与响应体。
func post(r *gin.Engine, token, body string) (int, string) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/system/user", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	r.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// uniqueToken 每个用例用独立 token，避免测试间共用防重键相互影响。
func uniqueToken(t *testing.T) string {
	t.Helper()
	return "tk-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + t.Name()
}

// TestSuccessBlocksRepeat 成功后保留键：interval 内的相同提交必须被挡。
// 对照 Java doAfterReturning「成功则不删 redis 数据」。
func TestSuccessBlocksRepeat(t *testing.T) {
	s := setupRedis(t)
	r := newEngine(s, func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "msg": "操作成功", "data": nil})
	}, WithInterval(time.Second))

	token := uniqueToken(t)

	code, body := post(r, token, `{"name":"张三"}`)
	if code != 200 || !strings.Contains(body, `"code":200`) {
		t.Fatalf("第 1 次应放行, code=%d body=%q", code, body)
	}

	code, body = post(r, token, `{"name":"张三"}`)
	if code != 200 {
		t.Errorf("防重 HTTP 码 = %d, want 200 (对齐 Java 非 429)", code)
	}
	if !strings.Contains(body, "不允许重复提交") {
		t.Errorf("防重文案应为 i18n 中文, got %q", body)
	}
}

// TestFailureAllowsRetry 业务失败应删键，用户可以立刻重试。
// 对照 Java doAfterReturning 里 code != SUCCESS 时 deleteRepeatKey。
func TestFailureAllowsRetry(t *testing.T) {
	s := setupRedis(t)
	r := newEngine(s, func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 500, "msg": "操作失败", "data": nil})
	}, WithInterval(time.Minute))

	token := uniqueToken(t)

	if code, body := post(r, token, `{"name":"张三"}`); code != 200 || !strings.Contains(body, `"code":500`) {
		t.Fatalf("第 1 次应到达 handler, code=%d body=%q", code, body)
	}
	// 第 1 次业务失败已删键，第 2 次必须仍能进 handler 而非被防重挡下。
	code, body := post(r, token, `{"name":"张三"}`)
	if strings.Contains(body, "不允许重复提交") {
		t.Errorf("业务失败后应允许重试, got %q", body)
	}
	if code != 200 || !strings.Contains(body, `"code":500`) {
		t.Errorf("第 2 次应再次到达 handler, code=%d body=%q", code, body)
	}
}

// TestPanicAllowsRetry handler panic 应删键并由 Recover 渲染 500，且不吐半截响应。
// 对照 Java doAfterThrowing。
func TestPanicAllowsRetry(t *testing.T) {
	s := setupRedis(t)
	hit := 0
	r := newEngine(s, func(c *gin.Context) {
		hit++
		panic("boom")
	}, WithInterval(time.Minute))

	token := uniqueToken(t)

	code, body := post(r, token, `{"name":"张三"}`)
	if code != 200 {
		t.Errorf("panic 后 HTTP 码 = %d, want 200 (Recover 统一渲染)", code)
	}
	if !strings.Contains(body, "未知异常") {
		t.Errorf("panic 应由 Recover 渲染统一文案, got %q", body)
	}

	post(r, token, `{"name":"张三"}`)
	if hit != 2 {
		t.Errorf("panic 后应删键允许重试, handler 命中 %d 次, want 2", hit)
	}
}

// TestDifferentParamsNotBlocked 入参不同不算重复提交。
func TestDifferentParamsNotBlocked(t *testing.T) {
	s := setupRedis(t)
	r := newEngine(s, func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "msg": "操作成功", "data": nil})
	}, WithInterval(time.Minute))

	token := uniqueToken(t)

	if _, body := post(r, token, `{"name":"张三"}`); strings.Contains(body, "不允许重复提交") {
		t.Fatalf("第 1 次不该被防重: %q", body)
	}
	if _, body := post(r, token, `{"name":"李四"}`); strings.Contains(body, "不允许重复提交") {
		t.Errorf("入参不同不应被防重: %q", body)
	}
}

// TestDifferentTokensNotBlocked 不同用户提交相同入参互不影响。
func TestDifferentTokensNotBlocked(t *testing.T) {
	s := setupRedis(t)
	r := newEngine(s, func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "msg": "操作成功", "data": nil})
	}, WithInterval(time.Minute))

	body := `{"name":"张三"}`
	if _, got := post(r, uniqueToken(t)+"-a", body); strings.Contains(got, "不允许重复提交") {
		t.Fatalf("用户 a 不该被防重: %q", got)
	}
	if _, got := post(r, uniqueToken(t)+"-b", body); strings.Contains(got, "不允许重复提交") {
		t.Errorf("用户 b 与 a 入参相同但 token 不同, 不应被防重: %q", got)
	}
}

// TestIntervalExpires 超过 interval 后应放行。
func TestIntervalExpires(t *testing.T) {
	s := setupRedis(t)
	r := newEngine(s, func(c *gin.Context) {
		c.JSON(200, gin.H{"code": 200, "msg": "操作成功", "data": nil})
	}, WithInterval(time.Second))

	token := uniqueToken(t)
	body := `{"name":"张三"}`

	if _, got := post(r, token, body); strings.Contains(got, "不允许重复提交") {
		t.Fatalf("第 1 次不该被防重: %q", got)
	}
	if _, got := post(r, token, body); !strings.Contains(got, "不允许重复提交") {
		t.Fatalf("interval 内第 2 次应被防重: %q", got)
	}

	time.Sleep(1200 * time.Millisecond)
	if _, got := post(r, token, body); strings.Contains(got, "不允许重复提交") {
		t.Errorf("interval 过后应放行: %q", got)
	}
}

// TestBodyReachesHandler 防重注解读过 body 后，handler 仍须能完整读到。
func TestBodyReachesHandler(t *testing.T) {
	s := setupRedis(t)

	var seen string
	r := newEngine(s, func(c *gin.Context) {
		var in struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(200, gin.H{"code": 500, "msg": err.Error(), "data": nil})
			return
		}
		seen = in.Name
		c.JSON(200, gin.H{"code": 200, "msg": "操作成功", "data": nil})
	}, WithInterval(time.Second))

	if _, body := post(r, uniqueToken(t), `{"name":"张三"}`); !strings.Contains(body, `"code":200`) {
		t.Fatalf("handler 应能绑定 body, got %q", body)
	}
	if seen != "张三" {
		t.Errorf("handler 读到的 name = %q, want 张三", seen)
	}
}

// TestRedisDownFailsOpen Redis 不可用时应放行而非阻断业务（可用性优先）。
// 与 pkg/ratelimiter 同款取舍，也是与 Java 的有意差异。
func TestRedisDownFailsOpen(t *testing.T) {
	// 指向一个必然连不上的端口，不需要真实 Redis，故不调 setupRedis。
	bad := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
	t.Cleanup(func() { _ = bad.Close() })
	s := &Submitter{tokenName: "Authorization", rdb: bad}

	hit := 0
	r := newEngine(s, func(c *gin.Context) {
		hit++
		c.JSON(200, gin.H{"code": 200, "msg": "操作成功", "data": nil})
	}, WithInterval(time.Minute))

	if _, body := post(r, "tk", `{"name":"张三"}`); !strings.Contains(body, `"code":200`) {
		t.Errorf("Redis 不可用应放行, got %q", body)
	}
	if hit != 1 {
		t.Errorf("handler 应被执行 1 次, got %d", hit)
	}
}
