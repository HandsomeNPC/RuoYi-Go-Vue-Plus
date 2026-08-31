package ratelimiter

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// newCtx 造一个指定路径与来源 IP 的 gin 上下文。
func newCtx(path, remoteAddr string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", path, nil)
	if remoteAddr != "" {
		c.Request.RemoteAddr = remoteAddr
	}
	return c
}

// TestDefaultOptions 全为零值时应回落到默认值。
func TestDefaultOptions(t *testing.T) {
	o := newOptions(0, 0, 0, 0, "")

	if got, want := o.window, 60*time.Second; got != want {
		t.Errorf("默认窗口 = %v, want %v", got, want)
	}
	if got, want := o.count, 100; got != want {
		t.Errorf("默认次数 = %d, want %d", got, want)
	}
	if got, want := o.limitType, LimitTypeGlobal; got != want {
		t.Errorf("默认类型 = %v, want %v", got, want)
	}
	if got, want := o.timeout, 24*time.Hour; got != want {
		t.Errorf("默认 timeout = %v, want %v", got, want)
	}
	if got, want := o.message, defaultMessage; got != want {
		t.Errorf("默认文案 = %q, want %q", got, want)
	}
	if o.keyFunc != nil {
		t.Error("默认不应有 keyFunc")
	}
}

// TestNewOptionsExplicit 显式传入的值应原样存入。
func TestNewOptionsExplicit(t *testing.T) {
	o := newOptions(10*time.Second, 2, LimitTypeCluster, time.Hour, "custom.code")

	if got, want := o.window, 10*time.Second; got != want {
		t.Errorf("窗口 = %v, want %v", got, want)
	}
	if got, want := o.count, 2; got != want {
		t.Errorf("次数 = %d, want %d", got, want)
	}
	if got, want := o.limitType, LimitTypeCluster; got != want {
		t.Errorf("类型 = %v, want %v", got, want)
	}
	if got, want := o.timeout, time.Hour; got != want {
		t.Errorf("timeout = %v, want %v", got, want)
	}
	if got, want := o.message, "custom.code"; got != want {
		t.Errorf("词条 = %q, want %q", got, want)
	}
}

// TestCombineKey 键格式：global:rate_limit:<路径>:[<IP>:|<实例ID>:]<动态维度>。
func TestCombineKey(t *testing.T) {
	l := &Limiter{instanceID: "inst-1"}

	cases := []struct {
		name      string
		path      string
		addr      string
		limitType LimitType
		fn        func(*gin.Context) string
		want      string
	}{
		{
			name: "默认全局限流,空动态维度保留结尾冒号",
			path: "/demo/rateLimiter/test",
			want: "global:rate_limit:/demo/rateLimiter/test:",
		},
		{
			name:      "IP 维度",
			path:      "/auth/code",
			addr:      "1.2.3.4:5678",
			limitType: LimitTypeIP,
			want:      "global:rate_limit:/auth/code:1.2.3.4:",
		},
		{
			name:      "实例维度",
			path:      "/demo/testcluster",
			limitType: LimitTypeCluster,
			want:      "global:rate_limit:/demo/testcluster:inst-1:",
		},
		{
			name: "动态维度(对照 SpEL #phoneNumber)",
			path: "/resource/sms/code",
			fn:   func(c *gin.Context) string { return c.Query("phoneNumber") },
			want: "global:rate_limit:/resource/sms/code:13800000000",
		},
		{
			name:      "IP + 动态维度",
			path:      "/demo/testObj",
			addr:      "9.9.9.9:1000",
			limitType: LimitTypeIP,
			fn:        func(c *gin.Context) string { return c.Query("value") },
			want:      "global:rate_limit:/demo/testObj:9.9.9.9:v1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := newOptions(0, 0, tc.limitType, 0, "")
			o.keyFunc = tc.fn

			path := tc.path
			if tc.fn != nil {
				path += "?phoneNumber=13800000000&value=v1"
			}
			got := l.combineKey(newCtx(path, tc.addr), o)
			if got != tc.want {
				t.Errorf("combineKey = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCombineKeyAlwaysUnderGlobalPrefix 限流键必须落在 global: 命名空间下。
func TestCombineKeyAlwaysUnderGlobalPrefix(t *testing.T) {
	l := &Limiter{instanceID: "x"}
	for _, lt := range []LimitType{LimitTypeGlobal, LimitTypeIP, LimitTypeCluster} {
		key := l.combineKey(newCtx("/p", "1.1.1.1:1"), newOptions(0, 0, lt, 0, ""))
		if !strings.HasPrefix(key, "global:rate_limit:") {
			t.Errorf("limitType=%v 的键前缀不对: %q", lt, key)
		}
	}
}

// TestNewInstanceIDUnique 实例 ID 必须唯一，否则集群限流会互相串档。
func TestNewInstanceIDUnique(t *testing.T) {
	seen := make(map[string]bool, 64)
	for range 64 {
		id := newInstanceID()
		if id == "" {
			t.Fatal("实例 ID 不应为空")
		}
		if seen[id] {
			t.Fatalf("实例 ID 重复: %s", id)
		}
		seen[id] = true
	}
}

// TestNewMemberSuffixUnique 成员后缀必须唯一，否则同毫秒并发请求会被 ZADD 覆盖成一次。
func TestNewMemberSuffixUnique(t *testing.T) {
	seen := make(map[string]bool, 256)
	for range 256 {
		s := newMemberSuffix()
		if seen[s] {
			t.Fatalf("成员后缀重复: %s", s)
		}
		seen[s] = true
	}
}

// TestGetPanicsWithoutInit 未 Init 直接用应 panic，对照 encrypt.getCrypto / redis.Client。
func TestGetPanicsWithoutInit(t *testing.T) {
	mu.Lock()
	saved := defaultLimiter
	defaultLimiter = nil
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		defaultLimiter = saved
		mu.Unlock()
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("未初始化时应 panic")
		}
	}()
	get()
}
