package vo

// SysUserImportVo 用户对象导入视图对象，对应 Java SysUserImportVo。
//
// excel tag 既是导入模板表头，也是读取时列定位依据（按表头文本匹配列）。
// 字段顺序即模板列序，与 Java 保持一致。
type SysUserImportVo struct {
	// UserID 用户ID，导入模板里的序号列，读取后落到 SysUserBo.UserID。
	UserID int64 `json:"userId" excel:"用户序号"`
	// DeptID 部门ID。Java 用 DeptExcelConverter 把部门名转回 id，Go 侧模板按部门ID填写、直读为 int64。
	DeptID int64 `json:"deptId" excel:"部门名称"`
	// UserName 用户账号。
	UserName string `json:"userName" excel:"用户账号"`
	// NickName 用户昵称。
	NickName string `json:"nickName" excel:"用户昵称"`
	// Email 用户邮箱。
	Email string `json:"email" excel:"用户邮箱"`
	// PhoneNumber 手机号码。
	PhoneNumber string `json:"phoneNumber" excel:"手机号码"`
	// Gender 用户性别（0男 1女 2未知）。
	Gender string `json:"gender" excel:"用户性别" excelDict:"0=男,1=女,2=未知"`
	// Status 账号状态（0正常 1停用）。
	Status string `json:"status" excel:"账号状态" excelDict:"0=正常,1=停用"`
}
