package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"ruoyi-go-vue-plus/pkg/config"
)

// unreachable 返回指向不存在数据库的配置。
func unreachable() config.DatasourceConfig {
	return config.DatasourceConfig{
		Driver:   config.DriverMySQL,
		Host:     "127.0.0.1",
		Port:     1,
		Username: "root",
		Password: "root",
		DBName:   "ry-vue",
		Params:   "charset=utf8mb4&parseTime=True&loc=Local&timeout=1s",
	}
}

// TestNewFailsWhenUnreachable 验证连不上库时返回错误。
func TestNewFailsWhenUnreachable(t *testing.T) {
	db, err := New(unreachable())
	if err == nil {
		t.Fatal("连不上数据库时 want error, got nil")
	}
	if db != nil {
		t.Errorf("失败时应返回 nil db, got %v", db)
	}
	if !strings.Contains(err.Error(), "database:") {
		t.Errorf("错误信息应带 database: 前缀, got %q", err)
	}
}

// TestInitFailsPanicsAndLeavesDefaultUnset 验证 Init 连不上库时 panic。
func TestInitFailsPanicsAndLeavesDefaultUnset(t *testing.T) {
	resetDefault(t)
	loadUnreachableConfig(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("连不上库时 Init 应 panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic 值应是 error，got %T: %v", r, r)
		}
		if !strings.Contains(err.Error(), "database:") {
			t.Errorf("panic 的错误信息应带 database: 前缀: %v", err)
		}
		mu.RLock()
		got := defaultDB
		mu.RUnlock()
		if got != nil {
			t.Error("Init 失败后 defaultDB 应仍为 nil")
		}
	}()
	Init()
}

// TestDBPanicsBeforeInit 验证未 Init 取用应 panic。
func TestDBPanicsBeforeInit(t *testing.T) {
	resetDefault(t)

	defer func() {
		if recover() == nil {
			t.Error("未初始化时 DB() 应 panic")
		}
	}()
	DB()
}

// TestCloseNilIsSafe 验证 Close/CloseDefault 对空实例安全。
func TestCloseNilIsSafe(t *testing.T) {
	if err := Close(nil); err != nil {
		t.Errorf("Close(nil) = %v, want nil", err)
	}
	resetDefault(t)
	CloseDefault()
}

// TestSingularTableNaming 验证表名单数。
func TestSingularTableNaming(t *testing.T) {
	ns := schema.NamingStrategy{SingularTable: true}

	tests := map[string]string{
		"SysUser":      "sys_user",
		"SysRole":      "sys_role",
		"SysUserRole":  "sys_user_role",
		"SysDictData":  "sys_dict_data",
		"SysOssConfig": "sys_oss_config",
	}
	for entity, want := range tests {
		if got := ns.TableName(entity); got != want {
			t.Errorf("TableName(%q) = %q, want %q", entity, got, want)
		}
	}
}

// TestLogLevelMapping 验证配置级别映射到 GORM 级别。
func TestLogLevelMapping(t *testing.T) {
	tests := map[string]logger.LogLevel{
		config.LogLevelSilent: logger.Silent,
		config.LogLevelError:  logger.Error,
		config.LogLevelWarn:   logger.Warn,
		config.LogLevelInfo:   logger.Info,
		"":                    logger.Warn, // 兜底
		"nonsense":            logger.Warn, // 兜底
	}
	for level, want := range tests {
		if got := logLevel(level); got != want {
			t.Errorf("logLevel(%q) = %v, want %v", level, got, want)
		}
	}
}

// TestNewLogger 验证 newLogger 能构造可用实例。
func TestNewLogger(t *testing.T) {
	if got := newLogger(unreachable()); got == nil {
		t.Error("newLogger 返回 nil")
	}
}

// TestSlowThreshold 验证慢 SQL 阈值换算。
func TestSlowThreshold(t *testing.T) {
	tests := map[int]time.Duration{
		0:    200 * time.Millisecond,
		-1:   200 * time.Millisecond,
		500:  500 * time.Millisecond,
		1000: time.Second,
	}
	for ms, want := range tests {
		d := config.DatasourceConfig{SlowThresholdMs: ms}
		if got := d.SlowThreshold(); got != want {
			t.Errorf("SlowThresholdMs=%d: SlowThreshold() = %v, want %v", ms, got, want)
		}
	}
}

// TestDatasourceLevelDefault 验证日志级别缺省为 warn。
func TestDatasourceLevelDefault(t *testing.T) {
	if got, want := (config.DatasourceConfig{}).Level(), config.LogLevelWarn; got != want {
		t.Errorf("Level() = %q, want %q", got, want)
	}
	if got, want := (config.DatasourceConfig{LogLevel: config.LogLevelInfo}).Level(),
		config.LogLevelInfo; got != want {
		t.Errorf("Level() = %q, want %q", got, want)
	}
}

// TestMaxIdleTime 验证空闲连接存活时长换算。
func TestMaxIdleTime(t *testing.T) {
	d := config.DatasourceConfig{ConnMaxIdleTime: 600}
	if got, want := d.MaxIdleTime(), 10*time.Minute; got != want {
		t.Errorf("MaxIdleTime() = %v, want %v", got, want)
	}
}

// resetDefault 清空包级实例。
func resetDefault(t *testing.T) {
	t.Helper()
	mu.Lock()
	defaultDB = nil
	mu.Unlock()
}

// unreachableYAML 一份能通过校验但数据源不可达的完整配置。
const unreachableYAML = `
server:
  name: test
  addr: ":1"
datasource:
  driver: mysql
  host: 127.0.0.1
  port: 1
  username: root
  password: root
  dbname: ry-vue
  params: charset=utf8mb4&parseTime=True&loc=Local&timeout=1s
redis:
  host: 127.0.0.1
  port: 6379
  db: 0
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
