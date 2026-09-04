package bo

// SysLoginInfoQueryBo 登录日志列表查询条件（query 参数）。
//
// 与 SysLoginInfoBo 分开而非复用：查询条件全部可选，SysLoginInfoBo 是记录落库的写入契约。
// 过滤口径：ipaddr/userName 走 like、status 走 eq、loginTime 走闭区间。
type SysLoginInfoQueryBo struct {
	IPAddr   string `form:"ipaddr"`
	UserName string `form:"userName"`
	Status   string `form:"status"`
	// BeginTime/EndTime 登录时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`
}
