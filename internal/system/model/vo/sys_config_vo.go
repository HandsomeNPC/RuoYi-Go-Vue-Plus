package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
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

// FromSysConfig 把实体转成 VO。
func FromSysConfig(c *systemmodel.SysConfig) *SysConfigVo {
	if c == nil {
		return nil
	}
	return &SysConfigVo{
		ConfigID:    c.ConfigID,
		ConfigName:  c.ConfigName,
		ConfigKey:   c.ConfigKey,
		ConfigValue: c.ConfigValue,
		ConfigType:  c.ConfigType,
		Remark:      c.Remark,
		CreateTime:  c.CreateTime,
	}
}
