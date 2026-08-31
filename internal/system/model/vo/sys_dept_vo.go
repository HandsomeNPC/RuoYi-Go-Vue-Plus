package vo

import (
	"time"
)

// SysDeptVo 部门视图对象，对应 Java SysDeptVo。
type SysDeptVo struct {
	DeptID   int64 `json:"deptId"`
	ParentID int64 `json:"parentId"`
	// ParentName 父部门名称，由 service 层回填。
	ParentName   string `json:"parentName"`
	Ancestors    string `json:"ancestors"`
	DeptName     string `json:"deptName"`
	DeptCategory string `json:"deptCategory"`
	OrderNum     int    `json:"orderNum"`
	Leader       int64  `json:"leader"`
	// LeaderName 负责人名称，由 service 层回填。
	LeaderName string `json:"leaderName"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
	// Status 部门状态（0正常 1停用）。
	Status     string     `json:"status"`
	CreateTime *time.Time `json:"createTime"`
	// Children 子部门，由 service 层构建部门树时回填。
	Children []*SysDeptVo `json:"children"`
}
