package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
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

// FromSysLoginInfo 把实体转成 VO。
func FromSysLoginInfo(l *systemmodel.SysLoginInfo) *SysLoginInfoVo {
	if l == nil {
		return nil
	}
	return &SysLoginInfoVo{
		InfoID:        l.InfoID,
		UserName:      l.UserName,
		ClientKey:     l.ClientKey,
		DeviceType:    l.DeviceType,
		Status:        l.Status,
		IPAddr:        l.IPAddr,
		LoginLocation: l.LoginLocation,
		Browser:       l.Browser,
		OS:            l.OS,
		Msg:           l.Msg,
		LoginTime:     l.LoginTime,
	}
}
