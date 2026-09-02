package vo

import (
	"time"
)

// SysLoginInfoVo 系统访问记录视图对象，对应 Java SysLoginInfoVo。
// excel tag 逐字抄 Java @ExcelProperty；deviceType/status 按 @ExcelDictFormat 转标签。
type SysLoginInfoVo struct {
	InfoID     int64  `json:"infoId" excel:"序号"`
	UserName   string `json:"userName" excel:"用户账号"`
	ClientKey  string `json:"clientKey" excel:"客户端"`
	DeviceType string `json:"deviceType" excel:"设备类型" excelDict:"pc=PC,android=安卓,ios=iOS,xcx=小程序"`
	// Status 登录状态（0成功 1失败）。
	Status        string     `json:"status" excel:"登录状态" excelDict:"0=成功,1=失败"`
	IPAddr        string     `json:"ipaddr" excel:"登录地址"`
	LoginLocation string     `json:"loginLocation" excel:"登录地点"`
	Browser       string     `json:"browser" excel:"浏览器"`
	OS            string     `json:"os" excel:"操作系统"`
	Msg           string     `json:"msg" excel:"提示消息"`
	LoginTime     *time.Time `json:"loginTime" excel:"访问时间"`
}
