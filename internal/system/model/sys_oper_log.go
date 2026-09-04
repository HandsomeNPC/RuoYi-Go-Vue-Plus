package model

import "time"

// SysOperLog 操作日志记录表（sys_oper_log）。
// 不继承 BaseEntity，无审计字段。
type SysOperLog struct {
	OperID int64  `gorm:"column:oper_id;primaryKey" json:"operId"`
	Title  string `gorm:"column:title" json:"title"`
	// BusinessType 业务类型（0其它 1新增 2修改 3删除）。
	BusinessType  int    `gorm:"column:business_type" json:"businessType"`
	Method        string `gorm:"column:method" json:"method"`
	RequestMethod string `gorm:"column:request_method" json:"requestMethod"`
	// OperatorType 操作类别（0其它 1后台用户 2手机端用户）。
	OperatorType int    `gorm:"column:operator_type" json:"operatorType"`
	OperName     string `gorm:"column:oper_name" json:"operName"`
	UserID       int64  `gorm:"column:user_id" json:"userId"`
	DeptID       int64  `gorm:"column:dept_id" json:"deptId"`
	DeptName     string `gorm:"column:dept_name" json:"deptName"`
	ClientKey    string `gorm:"column:client_key" json:"clientKey"`
	DeviceType   string `gorm:"column:device_type" json:"deviceType"`
	Browser      string `gorm:"column:browser" json:"browser"`
	OS           string `gorm:"column:os" json:"os"`
	OperURL      string `gorm:"column:oper_url" json:"operUrl"`
	OperIP       string `gorm:"column:oper_ip" json:"operIp"`
	OperLocation string `gorm:"column:oper_location" json:"operLocation"`
	OperParam    string `gorm:"column:oper_param" json:"operParam"`
	JSONResult   string `gorm:"column:json_result" json:"jsonResult"`
	// Status 操作状态（0正常 1异常）。
	Status   int        `gorm:"column:status" json:"status"`
	ErrorMsg string     `gorm:"column:error_msg" json:"errorMsg"`
	OperTime *time.Time `gorm:"column:oper_time" json:"operTime"`
	CostTime int64      `gorm:"column:cost_time" json:"costTime"`
}

// TableName 显式指定表名。
func (SysOperLog) TableName() string {
	return "sys_oper_log"
}
