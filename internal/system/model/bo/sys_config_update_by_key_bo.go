package bo

import "ruoyi-go-vue-plus/pkg/jsonx"

// SysConfigUpdateByKeyBo 按参数键名改配置的入参。
//
// 单开一型而非复用 SysConfigBo：前端只回传 configKey + configValue 两字段，
// 套上 SysConfigBo 的 configName required 会把这条正常调用直接卡死在参数校验上。
type SysConfigUpdateByKeyBo struct {
	ConfigKey string `json:"configKey" binding:"required,max=100"`
	// ConfigValue 用 LooseString 而非 string：开关类配置项（如
	// sys.oss.previewListResource）前端按布尔下发 true，而该列是 varchar、
	// jsonx 严格解码会直接拒收。
	ConfigValue jsonx.LooseString `json:"configValue" binding:"required,max=500"`
}
