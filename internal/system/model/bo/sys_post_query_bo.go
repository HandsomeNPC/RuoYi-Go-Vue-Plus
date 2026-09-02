package bo

// SysPostQueryBo 岗位列表查询条件（query 参数）。
//
// 与 SysPostBo 分开而非复用：查询条件全部可选，而 SysPostBo 的 binding:"required"
// 是新增场景的契约。Go 的 binding tag 没有校验分组概念，一个结构体只能有一套规则。
type SysPostQueryBo struct {
	PostCode     string `form:"postCode"`
	PostName     string `form:"postName"`
	PostCategory string `form:"postCategory"`
	// Status 状态（0正常 1停用）。
	Status string `form:"status"`
	// DeptID 单部门搜索：命中该部门下的岗位。
	DeptID int64 `form:"deptId"`
	// BelongDeptID 部门树搜索：命中该部门自身及其全部子部门下的岗位。
	// 由 service 解析成 DeptIDs 后交 repository 过滤，本身不直接落 SQL。
	BelongDeptID int64 `form:"belongDeptId"`
	// BeginTime/EndTime 创建时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`

	// DeptIDs 部门树搜索解析出的部门ID集，由 service 按 BelongDeptID 回填，不绑入参。
	DeptIDs []int64 `form:"-"`
}
