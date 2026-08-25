package model

// SysConfig 参数配置表（sys_config），对应 Java org.dromara.system.domain.SysConfig。
type SysConfig struct {
	ConfigID    int64  `gorm:"column:config_id;primaryKey" json:"configId"`
	ConfigName  string `gorm:"column:config_name" json:"configName"`
	ConfigKey   string `gorm:"column:config_key" json:"configKey"`
	ConfigValue string `gorm:"column:config_value" json:"configValue"`
	// ConfigType 系统内置（Y是 N否）。
	ConfigType string `gorm:"column:config_type" json:"configType"`
	Remark     string `gorm:"column:remark" json:"remark"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysConfig) TableName() string {
	return "sys_config"
}
