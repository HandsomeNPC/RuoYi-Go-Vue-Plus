package database

import (
	"log"
	"os"
	"time"

	gormlogger "gorm.io/gorm/logger"
)

// newLogger 按数据源配置构造 GORM 日志器。
func newLogger(cfg DatasourceConfig) gormlogger.Interface {
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

// logLevel 把配置里的字符串级别映射为 GORM 级别，非法值兜底 Warn。
func logLevel(level string) gormlogger.LogLevel {
	switch level {
	case LogLevelSilent:
		return gormlogger.Silent
	case LogLevelError:
		return gormlogger.Error
	case LogLevelInfo:
		return gormlogger.Info
	default:
		return gormlogger.Warn
	}
}

// pingTimeout 启动时探活数据库的超时时间。
const pingTimeout = 5 * time.Second
