package sms

import (
	"fmt"

	"github.com/spf13/viper"
)

// 阿里云短信的接入点默认值。
const (
	defaultSMSEndpoint = "dysmsapi.aliyuncs.com"
	defaultSMSRegionID = "cn-hangzhou"
)

// SMSConfig 短信配置，对应 yaml 的 sms 段。
//
// 只覆盖阿里云一家：sms4j 聚合多厂商，Go 生态无对应物，
// 故由 pkg/sms 的 Sender 接口留扩展点，配置段先按阿里云的参数形状定。
type SMSConfig struct {
	// Enabled 是否开启短信功能。关闭时发信接口返回提示而非报错。
	Enabled bool `mapstructure:"enabled"`
	// AccessKeyID 阿里云 AccessKey ID。
	AccessKeyID string `mapstructure:"accessKeyId"`
	// AccessKeySecret 阿里云 AccessKey Secret，参与签名。
	AccessKeySecret string `mapstructure:"accessKeySecret"`
	// SignName 短信签名，须与控制台已审核通过的签名完全一致。
	SignName string `mapstructure:"signName"`
	// TemplateCode 短信模板 CODE，形如 SMS_123456789。
	// 模板内容里的变量名须是 code，与 pkg/sms 拼的 TemplateParam 对应。
	TemplateCode string `mapstructure:"templateCode"`
	// Endpoint 服务接入点，一般不用改。
	Endpoint string `mapstructure:"endpoint"`
	// RegionID 地域，一般不用改。
	RegionID string `mapstructure:"regionId"`
}

// DefaultSMS 返回默认配置。默认关闭，理由同 DefaultMail。
func DefaultSMS() SMSConfig {
	return SMSConfig{
		Enabled:  false,
		Endpoint: defaultSMSEndpoint,
		RegionID: defaultSMSRegionID,
	}
}

// SetDefaults 把默认值铺给 viper。
func (c SMSConfig) SetDefaults(v *viper.Viper) {
	v.SetDefault("sms.enabled", c.Enabled)
	v.SetDefault("sms.accessKeyId", c.AccessKeyID)
	v.SetDefault("sms.accessKeySecret", c.AccessKeySecret)
	v.SetDefault("sms.signName", c.SignName)
	v.SetDefault("sms.templateCode", c.TemplateCode)
	v.SetDefault("sms.endpoint", c.Endpoint)
	v.SetDefault("sms.regionId", c.RegionID)
}

// Validate 校验短信配置。
func (c SMSConfig) Validate() error {
	// 关闭时其余项无需校验（同 mail/push）。
	if !c.Enabled {
		return nil
	}
	if c.AccessKeyID == "" {
		return fmt.Errorf("sms.accessKeyId 未配置")
	}
	if c.AccessKeySecret == "" {
		return fmt.Errorf("sms.accessKeySecret 未配置")
	}
	if c.SignName == "" {
		return fmt.Errorf("sms.signName 未配置")
	}
	if c.TemplateCode == "" {
		return fmt.Errorf("sms.templateCode 未配置")
	}
	if c.Endpoint == "" {
		return fmt.Errorf("sms.endpoint 未配置")
	}
	return nil
}
