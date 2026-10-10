package snowflake

// SnowflakeConfig 对应 yaml 的 snowflake 段。边界校验在 NewGenerator 内，本结构体不重复。
type SnowflakeConfig struct {
	WorkerID     int64 `mapstructure:"workerId"`     // 工作机器号，取值 0-31
	DatacenterID int64 `mapstructure:"datacenterId"` // 数据中心号，取值 0-31
}
