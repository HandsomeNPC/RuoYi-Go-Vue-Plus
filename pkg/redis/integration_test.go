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

// 真实 Redis 集成测试。默认跳过，需显式指定地址才运行：
//
//	RUOYI_TEST_REDIS_ADDR=127.0.0.1:6379 RUOYI_TEST_REDIS_PASSWORD=ruoyi123 \
//	  go test ./pkg/redis -run Integration -v
//
// 用 db=15 而非默认 0，避免污染业务数据；用例自行清理写入的 key。
func testConfig(t *testing.T) config.Redis {
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

	return config.Redis{
		Host:         host,
		Port:         port,
		Password:     os.Getenv("RUOYI_TEST_REDIS_PASSWORD"),
		DB:           15, // 测试专用库
		ClientName:   "ruoyi-go-test",
		PoolSize:     8,
		MinIdleConns: 1,
	}
}

// 真实环境下 New 能连通，且读写往返正常。
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
	// defer 后进先出：这里注册在 Close 之后，故先删 key 再关连接。
	// 用 t.Cleanup 会晚于所有 defer 执行，那时连接已关闭、删不掉。
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

// Init/Client/CloseDefault 的包级流程在真实环境下成立。
//
// Init 现在走 config.Get()，故先把 testConfig 写进临时 yaml 再 Load，
// 让 config.Get().Redis 指向测试库（db=15）。
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

	// 关闭后包级实例应已清空，再取用需重新 Init。
	mu.RLock()
	got := defaultClient
	mu.RUnlock()
	if got != nil {
		t.Error("CloseDefault 后 defaultClient 应为 nil")
	}
}

// loadRedisConfig 写入一份完整 yaml（redis 段取自 cfg）并 Load，
// 使 config.Get().Redis 返回给定配置。用于驱动走 config.Get() 的 Init()。
// datasource/server/jwt 段只是占位以通过 config 校验，本用例不碰它们。
func loadRedisConfig(t *testing.T, cfg config.Redis) {
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

// 密码错误必须报错，确认认证确实生效(测试环境 requirepass 已开启)。
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
