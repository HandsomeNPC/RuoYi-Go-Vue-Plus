package vo

import (
	"time"
)

// SysConfigVo 参数配置视图对象，对应 Java SysConfigVo。
type SysConfigVo struct {
	ConfigID    int64  `json:"configId"`
	ConfigName  string `json:"configName"`
	ConfigKey   string `json:"configKey"`
	ConfigValue string `json:"configValue"`
	// ConfigType 系统内置（Y是 N否）。
	ConfigType string     `json:"configType"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
}
