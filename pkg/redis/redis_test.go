package redis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ruoyi-go-vue-plus/pkg/config"
)

// unreachable 返回指向不存在 Redis 的配置。
func unreachable() config.RedisConfig {
	return config.RedisConfig{
		Host:          "127.0.0.1",
		Port:          1,
		DB:            0,
		DialTimeoutMs: 200,
	}
}

// TestNewFailsWhenUnreachable 验证连不上 Redis 时返回错误。
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

// TestInitFailsPanicsAndLeavesDefaultUnset 验证 Init 连不上时 panic。
func TestInitFailsPanicsAndLeavesDefaultUnset(t *testing.T) {
	resetDefault(t)
	loadUnreachableConfig(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("连不上 Redis 时 Init 应 panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic 值应是 error，got %T: %v", r, r)
		}
		if !strings.Contains(err.Error(), "redis:") {
			t.Errorf("panic 的错误信息应带 redis: 前缀: %v", err)
		}
		mu.RLock()
		got := defaultClient
		mu.RUnlock()
		if got != nil {
			t.Error("Init 失败后 defaultClient 应仍为 nil")
		}
	}()
	Init()
}

// TestClientPanicsBeforeInit 验证未 Init 取用应 panic。
func TestClientPanicsBeforeInit(t *testing.T) {
	resetDefault(t)

	defer func() {
		if recover() == nil {
			t.Error("未初始化时 Client() 应 panic")
		}
	}()
	Client()
}

// TestCloseNilIsSafe 验证 Close/CloseDefault 对空实例安全。
func TestCloseNilIsSafe(t *testing.T) {
	if err := Close(nil); err != nil {
		t.Errorf("Close(nil) = %v, want nil", err)
	}
	resetDefault(t)
	CloseDefault()
}

// TestTimeoutDefaults 验证超时项默认值。
func TestTimeoutDefaults(t *testing.T) {
	zero := config.RedisConfig{}
	if got, want := zero.DialTimeout(), 5*time.Second; got != want {
		t.Errorf("DialTimeout() = %v, want %v", got, want)
	}
	if got, want := zero.ReadTimeout(), 3*time.Second; got != want {
		t.Errorf("ReadTimeout() = %v, want %v", got, want)
	}
	if got, want := zero.WriteTimeout(), 3*time.Second; got != want {
		t.Errorf("WriteTimeout() = %v, want %v", got, want)
	}

	set := config.RedisConfig{DialTimeoutMs: 1000, ReadTimeoutMs: 500, WriteTimeoutMs: 1500}
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

// TestTimeoutNegativeFallsBack 验证负值兜底为默认。
func TestTimeoutNegativeFallsBack(t *testing.T) {
	r := config.RedisConfig{DialTimeoutMs: -1, ReadTimeoutMs: -100}
	if got, want := r.DialTimeout(), 5*time.Second; got != want {
		t.Errorf("DialTimeout() = %v, want %v", got, want)
	}
	if got, want := r.ReadTimeout(), 3*time.Second; got != want {
		t.Errorf("ReadTimeout() = %v, want %v", got, want)
	}
}

// TestMaxIdleTime 验证空闲连接存活时长换算。
func TestMaxIdleTime(t *testing.T) {
	r := config.RedisConfig{ConnMaxIdleTime: 600}
	if got, want := r.MaxIdleTime(), 10*time.Minute; got != want {
		t.Errorf("MaxIdleTime() = %v, want %v", got, want)
	}
}

// resetDefault 清空包级实例。
func resetDefault(t *testing.T) {
	t.Helper()
	mu.Lock()
	defaultClient = nil
	mu.Unlock()
}

// unreachableYAML 一份能通过校验但 Redis 不可达的完整配置。
const unreachableYAML = `
server:
  name: test
  addr: ":1"
datasource:
  driver: mysql
  host: 127.0.0.1
  port: 3306
  username: root
  password: root
  dbname: ry-vue
  params: charset=utf8mb4&parseTime=True&loc=Local
redis:
  host: 127.0.0.1
  port: 1
  db: 0
  dialTimeoutMs: 200
jwt:
  secret: test-secret
  expireMinutes: 720
  header: Authorization
`

// loadUnreachableConfig 写入临时 yaml 并 Load。
func loadUnreachableConfig(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte(unreachableYAML), 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	config.Load(path)
}
