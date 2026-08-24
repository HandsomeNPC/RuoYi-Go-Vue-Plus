package config

import "github.com/spf13/viper"

// Middleware 全局中间件配置，对应 yaml 的 middleware 段。
type Middleware struct {
	CORS           CORS           `mapstructure:"cors"`
	XSS            XSS            `mapstructure:"xss"`
	AccessLog      AccessLog      `mapstructure:"accessLog"`
	TraceID        TraceID        `mapstructure:"traceId"`
	RepeatableBody RepeatableBody `mapstructure:"repeatableBody"`
	I18n           I18n           `mapstructure:"i18n"`
	APIEncrypt     APIEncrypt     `mapstructure:"apiEncrypt"`
	Auth           Auth           `mapstructure:"auth"`
}

// DefaultMiddleware 返回全部中间件的默认配置。
func DefaultMiddleware() Middleware {
	return Middleware{
		CORS:           defaultCORS(),
		XSS:            defaultXSS(),
		AccessLog:      defaultAccessLog(),
		TraceID:        defaultTraceID(),
		RepeatableBody: defaultRepeatableBody(),
		I18n:           defaultI18n(),
		APIEncrypt:     defaultAPIEncrypt(),
		Auth:           defaultAuth(),
	}
}

// setMiddlewareDefaults 把默认值铺给 viper，必须在读配置文件之前调用。
func setMiddlewareDefaults(v *viper.Viper) {
	d := DefaultMiddleware()

	d.CORS.setDefaults(v)
	d.XSS.setDefaults(v)
	d.AccessLog.setDefaults(v)
	d.TraceID.setDefaults(v)
	d.RepeatableBody.setDefaults(v)
	d.I18n.setDefaults(v)
	d.APIEncrypt.setDefaults(v)
	d.Auth.setDefaults(v)
}

// validate 依次校验各中间件配置。
func (m Middleware) validate() error {
	validators := []func() error{
		m.CORS.validate,
		m.XSS.validate,
		m.AccessLog.validate,
		m.TraceID.validate,
		m.RepeatableBody.validate,
		m.I18n.validate,
		m.APIEncrypt.validate,
		m.Auth.validate,
	}
	for _, fn := range validators {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}
