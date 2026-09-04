package bo

// SysDeptBo 部门业务对象（入参）。
type SysDeptBo struct {
	DeptID       int64  `json:"deptId"`
	ParentID     int64  `json:"parentId"`
	DeptName     string `json:"deptName" binding:"required,max=30"`
	DeptCategory string `json:"deptCategory" binding:"omitempty,max=100"`
	OrderNum     int    `json:"orderNum" binding:"required"`
	Leader       int64  `json:"leader"`
	Phone        string `json:"phone" binding:"omitempty,max=11"`
	Email        string `json:"email" binding:"omitempty,email,max=50"`
	// Status 部门状态（0正常 1停用）。
	Status string `json:"status"`
	// BelongDeptID 归属部门id（部门树），不落 sys_dept，由 service 处理祖先链。
	BelongDeptID int64 `json:"belongDeptId"`
	// Params 请求参数袋，不落表。
	Params map[string]any `json:"params"`
}
