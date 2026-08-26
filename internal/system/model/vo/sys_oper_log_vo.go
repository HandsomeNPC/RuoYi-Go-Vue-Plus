package vo

import (
	"time"
)

// SysOperLogVo 操作日志记录视图对象，对应 Java SysOperLogVo。
type SysOperLogVo struct {
	OperID int64  `json:"operId"`
	Title  string `json:"title"`
	// BusinessType 业务类型（0其它 1新增 2修改 3删除）。
	BusinessType int `json:"businessType"`
	// BusinessTypes 业务类型数组，仅用于查询条件，不落 sys_oper_log，由 service 回填。
	BusinessTypes []int  `json:"businessTypes"`
	Method        string `json:"method"`
	RequestMethod string `json:"requestMethod"`
	// OperatorType 操作类别（0其它 1后台用户 2手机端用户）。
	OperatorType int    `json:"operatorType"`
	OperName     string `json:"operName"`
	UserID       int64  `json:"userId"`
	DeptID       int64  `json:"deptId"`
	DeptName     string `json:"deptName"`
	ClientKey    string `json:"clientKey"`
	DeviceType   string `json:"deviceType"`
	Browser      string `json:"browser"`
	OS           string `json:"os"`
	OperURL      string `json:"operUrl"`
	OperIP       string `json:"operIp"`
	OperLocation string `json:"operLocation"`
	OperParam    string `json:"operParam"`
	JSONResult   string `json:"jsonResult"`
	// Status 操作状态（0正常 1异常）。
	Status   int        `json:"status"`
	ErrorMsg string     `json:"errorMsg"`
	OperTime *time.Time `json:"operTime"`
	CostTime int64      `json:"costTime"`
}
