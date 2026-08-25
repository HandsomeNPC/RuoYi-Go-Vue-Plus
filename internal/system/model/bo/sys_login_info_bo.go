package bo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysLoginInfoBo 系统访问记录业务对象（入参），对应 Java SysLoginInfoBo。
type SysLoginInfoBo struct {
	InfoID        int64  `json:"infoId"`
	UserName      string `json:"userName"`
	ClientKey     string `json:"clientKey"`
	DeviceType    string `json:"deviceType"`
	IPAddr        string `json:"ipaddr"`
	LoginLocation string `json:"loginLocation"`
	Browser       string `json:"browser"`
	OS            string `json:"os"`
	// Status 登录状态（0成功 1失败）。
	Status    string     `json:"status"`
	Msg       string     `json:"msg"`
	LoginTime *time.Time `json:"loginTime"`
	// Params 请求参数袋，不落表。
	Params map[string]any `json:"params"`
}

// ToSysLoginInfo 把 BO 转成实体。
func (b *SysLoginInfoBo) ToSysLoginInfo() *systemmodel.SysLoginInfo {
	if b == nil {
		return nil
	}
	return &systemmodel.SysLoginInfo{
		InfoID:        b.InfoID,
		UserName:      b.UserName,
		ClientKey:     b.ClientKey,
		DeviceType:    b.DeviceType,
		Status:        b.Status,
		IPAddr:        b.IPAddr,
		LoginLocation: b.LoginLocation,
		Browser:       b.Browser,
		OS:            b.OS,
		Msg:           b.Msg,
		LoginTime:     b.LoginTime,
	}
}
