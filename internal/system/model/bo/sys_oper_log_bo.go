package bo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysOperLogBo 操作日志记录业务对象（入参），对应 Java SysOperLogBo。
type SysOperLogBo struct {
	OperID int64  `json:"operId"`
	Title  string `json:"title"`
	// BusinessType 业务类型（0其它 1新增 2修改 3删除）。
	BusinessType int `json:"businessType"`
	// BusinessTypes 业务类型数组，仅用于查询条件，不落 sys_oper_log。
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
	// Params 请求参数袋，不落表。
	Params map[string]any `json:"params"`
}

// ToSysOperLog 把 BO 转成实体。
func (b *SysOperLogBo) ToSysOperLog() *systemmodel.SysOperLog {
	if b == nil {
		return nil
	}
	return &systemmodel.SysOperLog{
		OperID:        b.OperID,
		Title:         b.Title,
		BusinessType:  b.BusinessType,
		Method:        b.Method,
		RequestMethod: b.RequestMethod,
		OperatorType:  b.OperatorType,
		OperName:      b.OperName,
		UserID:        b.UserID,
		DeptID:        b.DeptID,
		DeptName:      b.DeptName,
		ClientKey:     b.ClientKey,
		DeviceType:    b.DeviceType,
		Browser:       b.Browser,
		OS:            b.OS,
		OperURL:       b.OperURL,
		OperIP:        b.OperIP,
		OperLocation:  b.OperLocation,
		OperParam:     b.OperParam,
		JSONResult:    b.JSONResult,
		Status:        b.Status,
		ErrorMsg:      b.ErrorMsg,
		OperTime:      b.OperTime,
		CostTime:      b.CostTime,
	}
}
