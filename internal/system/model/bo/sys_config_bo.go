package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysConfigBo 参数配置业务对象（入参），对应 Java SysConfigBo。
type SysConfigBo struct {
	ConfigID    int64  `json:"configId"`
	ConfigName  string `json:"configName" binding:"required,max=100"`
	ConfigKey   string `json:"configKey" binding:"required,max=100"`
	ConfigValue string `json:"configValue" binding:"required,max=500"`
	// ConfigType 系统内置（Y是 N否）。
	ConfigType string `json:"configType"`
	Remark     string `json:"remark"`
	// Params 请求参数袋，不落表。
	Params map[string]any `json:"params"`
}

// ToSysConfig 把 BO 转成实体。
func (b *SysConfigBo) ToSysConfig() *systemmodel.SysConfig {
	if b == nil {
		return nil
	}
	return &systemmodel.SysConfig{
		ConfigID:    b.ConfigID,
		ConfigName:  b.ConfigName,
		ConfigKey:   b.ConfigKey,
		ConfigValue: b.ConfigValue,
		ConfigType:  b.ConfigType,
		Remark:      b.Remark,
	}
}
