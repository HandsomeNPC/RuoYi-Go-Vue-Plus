package config

import (
	"fmt"
	"time"
)

// DriverMySQL 支持的数据库驱动。
const DriverMySQL = "mysql"

// SQL 日志级别取值。
const (
	LogLevelSilent = "silent"
	LogLevelError  = "error"
	LogLevelWarn   = "warn"
	LogLevelInfo   = "info"
)

// Datasource MySQL 数据源配置。
type Datasource struct {
	Driver          string `mapstructure:"driver"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	Username        string `mapstructure:"username"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	Params          string `mapstructure:"params"`
	MaxIdleConns    int    `mapstructure:"maxIdleConns"`
	MaxOpenConns    int    `mapstructure:"maxOpenConns"`
	ConnMaxLifetime int    `mapstructure:"connMaxLifetime"` // 秒
	ConnMaxIdleTime int    `mapstructure:"connMaxIdleTime"` // 秒
	LogLevel        string `mapstructure:"logLevel"`        // silent/error/warn/info，缺省 warn
	SlowThresholdMs int    `mapstructure:"slowThresholdMs"` // 慢 SQL 阈值(毫秒)，缺省 200
}

// DSN 返回 GORM MySQL 连接串。
func (d Datasource) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		d.Username, d.Password, d.Host, d.Port, d.DBName, d.Params)
}

// MaxLifetime 返回连接最大存活时长。
func (d Datasource) MaxLifetime() time.Duration {
	return time.Duration(d.ConnMaxLifetime) * time.Second
}

// MaxIdleTime 返回空闲连接最大存活时长。
func (d Datasource) MaxIdleTime() time.Duration {
	return time.Duration(d.ConnMaxIdleTime) * time.Second
}

// SlowThreshold 返回慢 SQL 阈值，未配置时默认 200ms。
func (d Datasource) SlowThreshold() time.Duration {
	if d.SlowThresholdMs <= 0 {
		return 200 * time.Millisecond
	}
	return time.Duration(d.SlowThresholdMs) * time.Millisecond
}

// Level 返回规范化后的 SQL 日志级别，未配置时默认 warn。
func (d Datasource) Level() string {
	if d.LogLevel == "" {
		return LogLevelWarn
	}
	return d.LogLevel
}

// validate 校验数据源配置。
func (d Datasource) validate() error {
	if d.Driver != "" && d.Driver != DriverMySQL {
		return errInvalid("datasource.driver", "仅支持 "+DriverMySQL)
	}
	if d.Host == "" {
		return errMissing("datasource.host")
	}
	if d.DBName == "" {
		return errMissing("datasource.dbname")
	}
	if d.Port <= 0 {
		return errInvalid("datasource.port", "必须大于 0")
	}
	if d.MaxOpenConns > 0 && d.MaxIdleConns > d.MaxOpenConns {
		return errInvalid("datasource.maxIdleConns", "不能大于 maxOpenConns")
	}
	switch d.LogLevel {
	case "", LogLevelSilent, LogLevelError, LogLevelWarn, LogLevelInfo:
	default:
		return errInvalid("datasource.logLevel",
			"必须为 silent/error/warn/info 之一")
	}
	return nil
}
