package redis

import (
	"os"
	"path/filepath"
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

// Init 连不上 Redis 时必须 panic，且不污染包级实例。
//
// Init 现在走 config.Get()、失败直接 panic（对齐 config.Load 语义），故此处先
// Load 一份指向不存在端口的完整配置，再断言 Init() panic、且 defaultClient 仍为 nil。
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
	// CloseDefault 现在内部消化错误、不再返回，未初始化时调用应安全且不 panic。
	CloseDefault()
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

// unreachableYAML 一份能通过 config 校验、但 Redis 指向不存在端口的完整配置，
// 用于驱动 Init() 的失败分支。middleware/user 段由 viper 默认值补齐。
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
  port: 1  # 不可能有 Redis 监听
  db: 0
  dialTimeoutMs: 200
jwt:
  secret: test-secret
  expireMinutes: 720
  header: Authorization
`

// loadUnreachableConfig 写入临时 yaml 并 Load，使 config.Get().Redis 指向不可达实例。
func loadUnreachableConfig(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte(unreachableYAML), 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	config.Load(path)
}
