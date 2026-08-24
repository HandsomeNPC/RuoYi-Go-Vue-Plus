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

// 一份指向不存在的数据库的配置，用于验证失败路径。
func unreachable() config.Datasource {
	return config.Datasource{
		Driver:   config.DriverMySQL,
		Host:     "127.0.0.1",
		Port:     1, // 不可能有 MySQL 监听
		Username: "root",
		Password: "root",
		DBName:   "ry-vue",
		Params:   "charset=utf8mb4&parseTime=True&loc=Local&timeout=1s",
	}
}

// 连不上数据库时必须返回错误而不是可用实例，避免进程带坏连接启动。
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

// Init 连不上库时必须 panic，且不污染包级实例。
//
// Init 走 config.Get()、失败直接 panic（对齐 config.Load 语义），故此处先 Load
// 一份指向不存在端口的完整配置，再断言 Init() panic、且 defaultDB 仍为 nil。
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

// 未 Init 就取用应当 panic，把编排错误暴露在启动期。
func TestDBPanicsBeforeInit(t *testing.T) {
	resetDefault(t)

	defer func() {
		if recover() == nil {
			t.Error("未初始化时 DB() 应 panic")
		}
	}()
	DB()
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

// 表名必须是单数：实体 SysUser → 表 sys_user，与原项目表结构对齐。
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

// 配置里的字符串级别要正确映射到 GORM 级别，非法值兜底 Warn。
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

// newLogger 至少要能构造出可用实例，不 panic。
func TestNewLogger(t *testing.T) {
	if got := newLogger(unreachable()); got == nil {
		t.Error("newLogger 返回 nil")
	}
}

// 慢 SQL 阈值：未配置走默认 200ms，配置了按配置。
func TestSlowThreshold(t *testing.T) {
	tests := map[int]time.Duration{
		0:    200 * time.Millisecond,
		-1:   200 * time.Millisecond,
		500:  500 * time.Millisecond,
		1000: time.Second,
	}
	for ms, want := range tests {
		d := config.Datasource{SlowThresholdMs: ms}
		if got := d.SlowThreshold(); got != want {
			t.Errorf("SlowThresholdMs=%d: SlowThreshold() = %v, want %v", ms, got, want)
		}
	}
}

// 日志级别缺省为 warn。
func TestDatasourceLevelDefault(t *testing.T) {
	if got, want := (config.Datasource{}).Level(), config.LogLevelWarn; got != want {
		t.Errorf("Level() = %q, want %q", got, want)
	}
	if got, want := (config.Datasource{LogLevel: config.LogLevelInfo}).Level(),
		config.LogLevelInfo; got != want {
		t.Errorf("Level() = %q, want %q", got, want)
	}
}

// 空闲连接存活时长换算。
func TestMaxIdleTime(t *testing.T) {
	d := config.Datasource{ConnMaxIdleTime: 600}
	if got, want := d.MaxIdleTime(), 10*time.Minute; got != want {
		t.Errorf("MaxIdleTime() = %v, want %v", got, want)
	}
}

// resetDefault 清空包级实例，避免用例间互相影响。
func resetDefault(t *testing.T) {
	t.Helper()
	mu.Lock()
	defaultDB = nil
	mu.Unlock()
}

// unreachableYAML 一份能通过 config 校验、但数据源指向不存在端口的完整配置，
// 用于驱动 Init() 的失败分支。middleware/user 段由 viper 默认值补齐。
const unreachableYAML = `
server:
  name: test
  addr: ":1"
datasource:
  driver: mysql
  host: 127.0.0.1
  port: 1  # 不可能有 MySQL 监听
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

// loadUnreachableConfig 写入临时 yaml 并 Load，使 config.Get() 返回不可达数据源。
func loadUnreachableConfig(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte(unreachableYAML), 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	config.Load(path)
}
