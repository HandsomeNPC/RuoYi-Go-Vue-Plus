package bo

// SysPostBo 岗位信息业务对象（入参），对应 Java SysPostBo。
type SysPostBo struct {
	PostID int64 `json:"postId"`
	DeptID int64 `json:"deptId" binding:"required"`
	// BelongDeptID 归属部门id（部门树），不落 sys_post，由 service 处理。
	BelongDeptID int64  `json:"belongDeptId"`
	PostCode     string `json:"postCode" binding:"required,max=64"`
	PostName     string `json:"postName" binding:"required,max=50"`
	PostCategory string `json:"postCategory" binding:"omitempty,max=100"`
	PostSort     int    `json:"postSort" binding:"required"`
	// Status 状态（0正常 1停用）。
	Status string `json:"status"`
	Remark string `json:"remark"`
	// Params 请求参数袋，不落表。
	Params map[string]any `json:"params"`
}
