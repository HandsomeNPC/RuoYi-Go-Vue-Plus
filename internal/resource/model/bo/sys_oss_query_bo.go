package bo

// SysOssQueryBo OSS 列表查询条件（query 参数）。
//
// 与 SysOssBo 分开而非复用：查询条件全部可选，而写入 BO 的 binding 约束是新增场景的契约。
type SysOssQueryBo struct {
	FileName     string `form:"fileName"`
	OriginalName string `form:"originalName"`
	FileSuffix   string `form:"fileSuffix"`
	URL          string `form:"url"`
	Service      string `form:"service"`
	// CreateBy 上传人，0 视为不筛。
	CreateBy int64 `form:"createBy"`
	// BeginCreateTime/EndCreateTime 上传时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginCreateTime]/params[endCreateTime]，gin 按字面量匹配 form tag。
	// 注意键名带 CreateTime，与 sys_config 等模块的 beginTime/endTime 不同。
	BeginCreateTime string `form:"params[beginCreateTime]"`
	EndCreateTime   string `form:"params[endCreateTime]"`
}
