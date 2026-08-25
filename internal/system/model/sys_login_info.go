package model

import "time"

// SysLoginInfo 系统访问记录表（sys_login_info），对应 Java org.dromara.system.domain.SysLoginInfo。
// 不继承 BaseEntity，无审计字段。
type SysLoginInfo struct {
	InfoID     int64  `gorm:"column:info_id;primaryKey" json:"infoId"`
	UserName   string `gorm:"column:user_name" json:"userName"`
	ClientKey  string `gorm:"column:client_key" json:"clientKey"`
	DeviceType string `gorm:"column:device_type" json:"deviceType"`
	// Status 登录状态（0成功 1失败）。
	Status        string     `gorm:"column:status" json:"status"`
	IPAddr        string     `gorm:"column:ipaddr" json:"ipaddr"`
	LoginLocation string     `gorm:"column:login_location" json:"loginLocation"`
	Browser       string     `gorm:"column:browser" json:"browser"`
	OS            string     `gorm:"column:os" json:"os"`
	Msg           string     `gorm:"column:msg" json:"msg"`
	LoginTime     *time.Time `gorm:"column:login_time" json:"loginTime"`
}

// TableName 显式指定表名。
func (SysLoginInfo) TableName() string {
	return "sys_login_info"
}
