package bo

// SysConfigUpdateByKeyBo 按参数键名改配置的入参（对应 Java updateByKey 复用的 SysConfigBo）。
//
// 单开一型而非复用 SysConfigBo：Java 侧 updateByKey 唯独没挂 @Validated，
// 前端也只回传 configKey + configValue 两字段，套上 SysConfigBo 的
// configName required 会把这条正常调用直接卡死在参数校验上。
type SysConfigUpdateByKeyBo struct {
	ConfigKey   string `json:"configKey" binding:"required,max=100"`
	ConfigValue string `json:"configValue" binding:"required,max=500"`
}
