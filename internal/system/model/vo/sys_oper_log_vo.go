package vo

import (
	"time"
)

// SysOperLogVo 操作日志记录视图对象。
// excel tag 对应导出列；businessType/operatorType/status/deviceType
// 按 excelDict 转标签。BusinessTypes 仅查询条件用，导出不落列。
type SysOperLogVo struct {
	OperID int64  `json:"operId" excel:"日志主键"`
	Title  string `json:"title" excel:"操作模块"`
	// BusinessType 业务类型（0其它 1新增 2修改 3删除）。
	BusinessType int `json:"businessType" excel:"业务类型" excelDict:"0=其他,1=新增,2=修改,3=删除,4=授权,5=导出,6=导入,7=强退,8=生成代码,9=清空数据"`
	// BusinessTypes 业务类型数组，仅用于查询条件，不落 sys_oper_log，由 service 回填。
	BusinessTypes []int  `json:"businessTypes"`
	Method        string `json:"method" excel:"请求方法"`
	RequestMethod string `json:"requestMethod" excel:"请求方式"`
	// OperatorType 操作类别（0其它 1后台用户 2手机端用户）。
	OperatorType int    `json:"operatorType" excel:"操作类别" excelDict:"0=其它,1=后台用户,2=手机端用户"`
	OperName     string `json:"operName" excel:"操作人员"`
	UserID       int64  `json:"userId" excel:"操作用户ID"`
	DeptID       int64  `json:"deptId" excel:"操作部门ID"`
	DeptName     string `json:"deptName" excel:"部门名称"`
	ClientKey    string `json:"clientKey" excel:"客户端"`
	DeviceType   string `json:"deviceType" excel:"设备类型" excelDict:"pc=PC,android=安卓,ios=iOS,xcx=小程序"`
	Browser      string `json:"browser" excel:"浏览器"`
	OS           string `json:"os" excel:"操作系统"`
	OperURL      string `json:"operUrl" excel:"请求地址"`
	OperIP       string `json:"operIp" excel:"操作地址"`
	OperLocation string `json:"operLocation" excel:"操作地点"`
	OperParam    string `json:"operParam" excel:"请求参数"`
	JSONResult   string `json:"jsonResult" excel:"返回参数"`
	// Status 操作状态（0正常 1异常）。
	Status   int        `json:"status" excel:"状态" excelDict:"0=成功,1=失败"`
	ErrorMsg string     `json:"errorMsg" excel:"错误消息"`
	OperTime *time.Time `json:"operTime" excel:"操作时间"`
	CostTime int64      `json:"costTime" excel:"消耗时间"`
}
