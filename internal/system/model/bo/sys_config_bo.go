package bo

// SysConfigBo 参数配置业务对象（入参）。
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
