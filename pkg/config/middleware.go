// 本文件只放中间件配置的**汇总与编排**。
//
// 每个中间件的结构体、默认值、SetDefault 键、校验各自成文件：
// middleware_cors.go / middleware_xss.go / middleware_accesslog.go /
// middleware_trace.go / middleware_body.go / middleware_i18n.go /
// middleware_encrypt.go，
// 与 server.go / jwt.go / redis.go / datasource.go「一个子配置一个文件」的约定一致。
//
// 新增一个中间件配置要动四处，都在同一个文件里加一行：
// Middleware 的字段、DefaultMiddleware、setMiddlewareDefaults、validate。
package config

import "github.com/spf13/viper"

// Middleware 全局中间件配置，对应 yaml 的 middleware 段。
//
// 各中间件的无参构造函数（middleware.CORS() 等）从 Get().Middleware 读取本结构，
// 所以进程必须先 Load 再注册中间件。想绕开全局配置时用
// middleware.XxxWithConfig(cfg) 显式传入。
//
// 有意**没有** enabled 开关：Go 侧「注册」就是 middleware.Register 里那行
// r.Use(XSS())，不写即关闭。再加一个布尔开关只会造出「注册了但不生效」这种
// 要翻两处配置才能确诊的状态。
//
// 唯一例外是 APIEncrypt —— 它不生效时请求会被当明文交给 handler、报出一个
// 与真实原因无关的参数错误，必须能区分「没开」和「开了但密钥错」。
// 详见 APIEncrypt 的说明。
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
//
// 与 setMiddlewareDefaults 铺给 viper 的值必须**逐字段一致**，
// 由 TestMiddlewareDefaultsMatchSetDefault 锁住 —— 两者一旦分叉，
// 「yaml 没写该项」和「显式构造默认配置」就会得到不同行为。
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

// setMiddlewareDefaults 把默认值铺给 viper，必须在读配置文件**之前**调用。
//
// 不能只靠 Unmarshal 后补默认值：viper 对缺失的键给零值，而多数字段的零值
// 是**有意义但错误**的配置 —— CORS.AllowedOriginPatterns 为空切片意味着
// 「拒绝所有来源」而非「用默认值」，yaml 里漏写 middleware 段会让跨域全挂。
// 铺默认值让「没写」与「写了默认值」走同一条路。
//
// 切片类字段同理不能靠 <=0 之类的兜底判断：调用方可能就是想配空列表
// （如 xss.excludeUrls: []），那与「没配」必须可区分。
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
//
// 只拦「取值本身讲不通」的情况。语义上的空列表（如不配 excludeUrls）
// 是合法的，不在这里报错。
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
