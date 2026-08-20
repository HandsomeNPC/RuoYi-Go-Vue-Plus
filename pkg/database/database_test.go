package database

import (
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

// Init 失败时不得污染包级实例。
func TestInitFailsLeavesDefaultUnset(t *testing.T) {
	resetDefault(t)

	if err := Init(unreachable()); err == nil {
		t.Fatal("want error, got nil")
	}

	mu.RLock()
	got := defaultDB
	mu.RUnlock()
	if got != nil {
		t.Error("Init 失败后 defaultDB 应仍为 nil")
	}
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
	if err := CloseDefault(); err != nil {
		t.Errorf("未初始化时 CloseDefault() = %v, want nil", err)
	}
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
