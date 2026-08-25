package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
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
	Children []SysDeptVo `json:"children"`
}

// FromSysDept 把实体转成 VO。
func FromSysDept(d *systemmodel.SysDept) *SysDeptVo {
	if d == nil {
		return nil
	}
	return &SysDeptVo{
		DeptID:       d.DeptID,
		ParentID:     d.ParentID,
		Ancestors:    d.Ancestors,
		DeptName:     d.DeptName,
		DeptCategory: d.DeptCategory,
		OrderNum:     d.OrderNum,
		Leader:       d.Leader,
		Phone:        d.Phone,
		Email:        d.Email,
		Status:       d.Status,
		CreateTime:   d.CreateTime,
	}
}
