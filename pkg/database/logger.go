package database

import (
	"log"
	"os"
	"time"

	gormlogger "gorm.io/gorm/logger"

	"ruoyi-go-vue-plus/pkg/config"
)

// newLogger 按数据源配置构造 GORM 日志器。
//
// 日志级别与慢 SQL 阈值来自配置；RecordNotFound 属业务正常分支，
// 由 service 层判断，不当错误打印。
func newLogger(cfg config.Datasource) gormlogger.Interface {
	return gormlogger.New(
		log.New(os.Stdout, "[sql] ", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             cfg.SlowThreshold(),
			LogLevel:                  logLevel(cfg.Level()),
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}

// logLevel 把配置里的字符串级别映射为 GORM 级别。
// 非法值已由 config 校验拦截，这里兜底为 Warn。
func logLevel(level string) gormlogger.LogLevel {
	switch level {
	case config.LogLevelSilent:
		return gormlogger.Silent
	case config.LogLevelError:
		return gormlogger.Error
	case config.LogLevelInfo:
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

// pingTimeout 启动时探活数据库的超时时间。
const pingTimeout = 5 * time.Second
