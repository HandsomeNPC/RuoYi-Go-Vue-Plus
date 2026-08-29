package config

import "github.com/spf13/viper"

// SnowflakeMaxID 数据中心号 / 工作机器号的上界（5 位，0-31）。
const SnowflakeMaxID = 31

// SnowflakeConfig 雪花 ID 生成器配置，对应 yaml 的 snowflake 段。
//
// 各业务表主键均为 bigint 且无 auto_increment，由应用层发号（对照 Java
// MyBatis-Plus 的 ASSIGN_ID）。多进程共用同一库时，**每个进程必须配到不同的
// workerId**，否则同一毫秒内可能撞号，故该项放在 <module>.yaml 而非 application.yaml。
type SnowflakeConfig struct {
	// WorkerID 工作机器号，取值 0-SnowflakeMaxID。
	WorkerID int64 `mapstructure:"workerId"`

	// DatacenterID 数据中心号，取值 0-SnowflakeMaxID。
	DatacenterID int64 `mapstructure:"datacenterId"`
}

// DefaultSnowflake 返回雪花 ID 默认配置。
func DefaultSnowflake() SnowflakeConfig {
	return SnowflakeConfig{
		WorkerID:     0,
		DatacenterID: 0,
	}
}

// setDefaults 把默认值铺给 viper。
func (s SnowflakeConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("snowflake.workerId", s.WorkerID)
	v.SetDefault("snowflake.datacenterId", s.DatacenterID)
}

// validate 校验雪花 ID 配置。
func (s SnowflakeConfig) validate() error {
	if s.WorkerID < 0 || s.WorkerID > SnowflakeMaxID {
		return errInvalid("snowflake.workerId", "必须在 0-31 之间")
	}
	if s.DatacenterID < 0 || s.DatacenterID > SnowflakeMaxID {
		return errInvalid("snowflake.datacenterId", "必须在 0-31 之间")
	}
	return nil
}
