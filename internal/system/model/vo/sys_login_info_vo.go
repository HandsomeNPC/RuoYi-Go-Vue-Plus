package vo

import (
	"time"
)

// SysLoginInfoVo 系统访问记录视图对象，对应 Java SysLoginInfoVo。
type SysLoginInfoVo struct {
	InfoID     int64  `json:"infoId"`
	UserName   string `json:"userName"`
	ClientKey  string `json:"clientKey"`
	DeviceType string `json:"deviceType"`
	// Status 登录状态（0成功 1失败）。
	Status        string     `json:"status"`
	IPAddr        string     `json:"ipaddr"`
	LoginLocation string     `json:"loginLocation"`
	Browser       string     `json:"browser"`
	OS            string     `json:"os"`
	Msg           string     `json:"msg"`
	LoginTime     *time.Time `json:"loginTime"`
}
