package config

import (
	"fmt"
	"time"
)

// Datasource MySQL 数据源配置。各进程共用同一数据库。
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

// validate 校验数据源配置。
func (d Datasource) validate() error {
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
	return nil
}
