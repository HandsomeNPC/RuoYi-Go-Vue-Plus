package bo

// SysDeptQueryBo 部门列表查询条件（query 参数）。
//
// 与 SysDeptBo 分开而非复用：查询条件全部可选，而 SysDeptBo 的 binding:"required"
// 是新增场景的契约。Go 的 binding tag 没有校验分组概念，一个结构体只能有一套规则。
type SysDeptQueryBo struct {
	// DeptID/ParentID/BelongDeptID 取 0 视为不筛。
	//
	// Java 用 Long 的 null 区分"未传"与"显式传 0"，Go 的 int64 两者都塌成 0。
	// 这里选择 0 即不筛：parent_id = 0 的根部门在初始化数据里只有一条，
	// 且前端的上级部门下拉永远给真实主键，取不到可观测差异。
	DeptID       int64  `form:"deptId"`
	ParentID     int64  `form:"parentId"`
	DeptName     string `form:"deptName"`
	DeptCategory string `form:"deptCategory"`
	// Status 部门状态（0正常 1停用）。
	Status string `form:"status"`
	// BelongDeptID 归属部门（部门树搜索）：命中该部门自身及其全部子部门。
	BelongDeptID int64 `form:"belongDeptId"`
	// BeginTime/EndTime 创建时间区间，闭区间，两端须同时给出才生效。
	//
	// tag 里的方括号不是笔误：前端 fieldMappingTime 把日期区间摊平成
	// params[beginTime]/params[endTime] 两个 query 参数，gin 按字面量匹配 form tag。
	BeginTime string `form:"params[beginTime]"`
	EndTime   string `form:"params[endTime]"`
}
