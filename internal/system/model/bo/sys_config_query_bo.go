package bo

// SysConfigQueryBo 参数配置列表查询条件（query 参数）。
//
// 与 SysConfigBo 分开而非复用：查询条件全部可选，而 SysConfigBo 的 binding:"required"
// 是新增/修改场景的契约（同 SysClientQueryBo 的取舍）。
type SysConfigQueryBo struct {
	ConfigName string `form:"configName"`
	ConfigKey  string `form:"configKey"`
	// ConfigType 系统内置（Y是 N否）。
	ConfigType string `form:"configType"`
	// BeginTime/EndTime 创建时间区间，闭区间，两端须同时给出才生效
	// （对齐 Java betweenParams 的 begin != null && end != null）。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag，
	// 故此处直接接成两个平铺字段，不引入 map[string]any 的参数袋。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`
}
