package vo

import (
	"time"
)

// SysPostVo 岗位信息视图对象，对应 Java SysPostVo。
type SysPostVo struct {
	// excel tag 的值逐字抄 Java @ExcelProperty，无 tag 的字段不导出。
	PostID       int64  `json:"postId" excel:"岗位序号"`
	DeptID       int64  `json:"deptId" excel:"部门id"`
	PostCode     string `json:"postCode" excel:"岗位编码"`
	PostName     string `json:"postName" excel:"岗位名称"`
	PostCategory string `json:"postCategory" excel:"类别编码"`
	PostSort     int    `json:"postSort" excel:"岗位排序"`
	// Status 状态（0正常 1停用）。
	// 导出时按 excelDict 转标签，对齐 Java @ExcelDictFormat(dictType = "sys_normal_disable")。
	Status     string     `json:"status" excel:"状态" excelDict:"0=正常,1=停用"`
	Remark     string     `json:"remark" excel:"备注"`
	CreateTime *time.Time `json:"createTime" excel:"创建时间"`
	// DeptName 部门名，由翻译层按 DEPT_ID_TO_NAME 从 DeptId 回填。
	DeptName string `json:"deptName"`
}
