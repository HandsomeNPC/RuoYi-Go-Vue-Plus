package bo

// SysOperLogQueryBo 操作日志列表查询条件（query 参数）。
//
// 与 SysOperLogBo 分开而非复用：查询条件全部可选，SysOperLogBo 是记录落库的写入契约。
// 过滤口径：operIp/title/operName/browser/os 走 like、businessType/status 走 eq、
// businessTypes 走 in、operTime 走闭区间。
//
// Status 取 string 而非 int：操作状态 0=成功 是合法筛选项，Go 的 int 零值无法区分
// 「筛成功」与「未传」，用空串表示不筛，与 SysLoginInfoQueryBo 的处理一致。
type SysOperLogQueryBo struct {
	OperIP string `form:"operIp"`
	Title  string `form:"title"`
	// BusinessType 单值业务类型。按 businessType > 0：0=其他 不参与单值过滤，
	// 要按"其他"筛得走 BusinessTypes。
	BusinessType  int    `form:"businessType"`
	BusinessTypes []int  `form:"businessTypes"`
	Status        string `form:"status"`
	OperName      string `form:"operName"`
	UserID        int64  `form:"userId"`
	DeptID        int64  `form:"deptId"`
	ClientKey     string `form:"clientKey"`
	DeviceType    string `form:"deviceType"`
	Browser       string `form:"browser"`
	OS            string `form:"os"`
	// BeginTime/EndTime 操作时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`
}
