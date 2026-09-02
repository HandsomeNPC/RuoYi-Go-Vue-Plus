package vo

import (
	"time"
)

// SysUserExportVo 用户对象导出视图对象，对应 Java SysUserExportVo。
//
// excel tag 的值逐字抄 Java @ExcelProperty，无 tag 的字段不导出。
// 字段顺序即导出列序，与 Java 保持一致。
type SysUserExportVo struct {
	UserID   int64  `json:"userId" excel:"用户序号"`
	UserName string `json:"userName" excel:"用户账号"`
	// DeptID 导出时回填成部门名称显示（Java 用 DeptExcelConverter，Go 侧由 service 直接写 DeptName 列）。
	// 这里 DeptName 独立成列承接部门名，DeptID 本身不单列导出，对齐 Java「部门名称」一列。
	DeptID      int64  `json:"deptId"`
	DeptName    string `json:"deptName" excel:"部门名称"`
	NickName    string `json:"nickName" excel:"用户昵称"`
	Email       string `json:"email" excel:"用户邮箱"`
	PhoneNumber string `json:"phoneNumber" excel:"手机号码"`
	// Gender 用户性别（0男 1女 2未知），导出按 excelDict 转标签，对齐 Java @ExcelDictFormat(dictType="sys_user_gender")。
	Gender string `json:"gender" excel:"用户性别" excelDict:"0=男,1=女,2=未知"`
	// Status 账号状态（0正常 1停用），导出按 excelDict 转标签，对齐 Java @ExcelDictFormat(dictType="sys_normal_disable")。
	Status    string     `json:"status" excel:"账号状态" excelDict:"0=正常,1=停用"`
	LoginIP   string     `json:"loginIp" excel:"最后登录IP"`
	LoginDate *time.Time `json:"loginDate" excel:"最后登录时间"`
	// LeaderName 部门负责人名，由导出 service 按部门 leader 用户回填。
	LeaderName string `json:"leaderName" excel:"部门负责人"`
}
