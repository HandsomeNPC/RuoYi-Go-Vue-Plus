package redis

import (
	"strings"
	"testing"
	"time"

	"ruoyi-go-vue-plus/pkg/config"
)

// 一份指向不存在的 Redis 的配置，用于验证失败路径。
func unreachable() config.Redis {
	return config.Redis{
		Host:          "127.0.0.1",
		Port:          1, // 不可能有 Redis 监听
		DB:            0,
		DialTimeoutMs: 200,
	}
}

// 连不上 Redis 时必须返回错误而不是可用实例，避免进程带坏连接启动。
func TestNewFailsWhenUnreachable(t *testing.T) {
	client, err := New(unreachable())
	if err == nil {
		t.Fatal("连不上 Redis 时 want error, got nil")
	}
	if client != nil {
		t.Errorf("失败时应返回 nil client, got %v", client)
	}
	if !strings.Contains(err.Error(), "redis:") {
		t.Errorf("错误信息应带 redis: 前缀, got %q", err)
	}
}

// Init 失败时不得污染包级实例。
func TestInitFailsLeavesDefaultUnset(t *testing.T) {
	resetDefault(t)

	if err := Init(unreachable()); err == nil {
		t.Fatal("want error, got nil")
	}

	mu.RLock()
	got := defaultClient
	mu.RUnlock()
	if got != nil {
		t.Error("Init 失败后 defaultClient 应仍为 nil")
	}
}

// 未 Init 就取用应当 panic，把编排错误暴露在启动期。
func TestClientPanicsBeforeInit(t *testing.T) {
	resetDefault(t)

	defer func() {
		if recover() == nil {
			t.Error("未初始化时 Client() 应 panic")
		}
	}()
	Client()
}

// Close/CloseDefault 对空实例应当安全。
func TestCloseNilIsSafe(t *testing.T) {
	if err := Close(nil); err != nil {
		t.Errorf("Close(nil) = %v, want nil", err)
	}
	resetDefault(t)
	if err := CloseDefault(); err != nil {
		t.Errorf("未初始化时 CloseDefault() = %v, want nil", err)
	}
}

// 超时项：未配置走默认，配置了按配置换算。
func TestTimeoutDefaults(t *testing.T) {
	zero := config.Redis{}
	if got, want := zero.DialTimeout(), 5*time.Second; got != want {
		t.Errorf("DialTimeout() = %v, want %v", got, want)
	}
	if got, want := zero.ReadTimeout(), 3*time.Second; got != want {
		t.Errorf("ReadTimeout() = %v, want %v", got, want)
	}
	if got, want := zero.WriteTimeout(), 3*time.Second; got != want {
		t.Errorf("WriteTimeout() = %v, want %v", got, want)
	}

	set := config.Redis{DialTimeoutMs: 1000, ReadTimeoutMs: 500, WriteTimeoutMs: 1500}
	if got, want := set.DialTimeout(), time.Second; got != want {
		t.Errorf("DialTimeout() = %v, want %v", got, want)
	}
	if got, want := set.ReadTimeout(), 500*time.Millisecond; got != want {
		t.Errorf("ReadTimeout() = %v, want %v", got, want)
	}
	if got, want := set.WriteTimeout(), 1500*time.Millisecond; got != want {
		t.Errorf("WriteTimeout() = %v, want %v", got, want)
	}
}

// 负值等同未配置，兜底为默认值。
func TestTimeoutNegativeFallsBack(t *testing.T) {
	r := config.Redis{DialTimeoutMs: -1, ReadTimeoutMs: -100}
	if got, want := r.DialTimeout(), 5*time.Second; got != want {
		t.Errorf("DialTimeout() = %v, want %v", got, want)
	}
	if got, want := r.ReadTimeout(), 3*time.Second; got != want {
		t.Errorf("ReadTimeout() = %v, want %v", got, want)
	}
}

// 空闲连接存活时长换算。
func TestMaxIdleTime(t *testing.T) {
	r := config.Redis{ConnMaxIdleTime: 600}
	if got, want := r.MaxIdleTime(), 10*time.Minute; got != want {
		t.Errorf("MaxIdleTime() = %v, want %v", got, want)
	}
}

// resetDefault 清空包级实例，避免用例间互相影响。
func resetDefault(t *testing.T) {
	t.Helper()
	mu.Lock()
	defaultClient = nil
	mu.Unlock()
}
