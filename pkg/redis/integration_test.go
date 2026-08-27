package redis

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"ruoyi-go-vue-plus/pkg/config"
)

// testConfig 返回真实 Redis 测试配置，未设置地址时跳过。
func testConfig(t *testing.T) config.RedisConfig {
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

	return config.RedisConfig{
		Host:         host,
		Port:         port,
		Password:     os.Getenv("RUOYI_TEST_REDIS_PASSWORD"),
		DB:           15, // 测试专用库
		ClientName:   "ruoyi-go-test",
		PoolSize:     8,
		MinIdleConns: 1,
	}
}

// TestIntegrationNewAndRoundTrip 验证真实环境下 New 与读写往返。
func TestIntegrationNewAndRoundTrip(t *testing.T) {
	client, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() {
		if err := Close(client); err != nil {
			t.Errorf("Close: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const key = "ruoyi:test:roundtrip"
	// defer 后进先出：先删 key 再关连接。
	defer func() {
		if err := client.Del(ctx, key).Err(); err != nil {
			t.Errorf("清理 key 失败: %v", err)
		}
	}()

	if err := client.Set(ctx, key, "pong", time.Minute).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if want := "pong"; got != want {
		t.Errorf("Get(%q) = %q, want %q", key, got, want)
	}
}

// TestIntegrationInitAndClose 验证真实环境下的 Init/Client/CloseDefault 流程。
func TestIntegrationInitAndClose(t *testing.T) {
	cfg := testConfig(t)
	loadRedisConfig(t, cfg)
	resetDefault(t)

	Init()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Client().Ping(ctx).Err(); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	CloseDefault()

	// 关闭后包级实例应已清空。
	mu.RLock()
	got := defaultClient
	mu.RUnlock()
	if got != nil {
		t.Error("CloseDefault 后 defaultClient 应为 nil")
	}
}

// loadRedisConfig 写入完整 yaml（redis 段取自 cfg）并 Load。
func loadRedisConfig(t *testing.T, cfg config.RedisConfig) {
	t.Helper()
	yaml := fmt.Sprintf(`
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
  host: %s
  port: %d
  password: %s
  db: %d
  clientName: %s
  poolSize: %d
  minIdleConns: %d
  dialTimeoutMs: %d
jwt:
  secret: test-secret
  expireMinutes: 720
  header: Authorization
`,
		cfg.Host, cfg.Port, strconv.Quote(cfg.Password), cfg.DB,
		strconv.Quote(cfg.ClientName), cfg.PoolSize, cfg.MinIdleConns, cfg.DialTimeoutMs,
	)
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	config.Load(path)
}

// TestIntegrationWrongPasswordFails 验证密码错误时报错。
func TestIntegrationWrongPasswordFails(t *testing.T) {
	cfg := testConfig(t)
	if cfg.Password == "" {
		t.Skip("未设置密码，跳过认证校验")
	}
	cfg.Password = "definitely-wrong-password"

	if _, err := New(cfg); err == nil {
		t.Error("密码错误时 want error, got nil")
	}
}
