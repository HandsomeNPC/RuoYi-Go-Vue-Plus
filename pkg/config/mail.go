package config

import "github.com/spf13/viper"

// 默认 SMTP 参数。
const (
	// defaultMailPort 465 是 SMTP over SSL 的约定端口，与 defaultMailSSL 配套。
	defaultMailPort = 465
	// defaultMailSSL 默认走 SSL 直连而非 STARTTLS：国内邮箱（QQ/163/阿里）
	// 的 465 端口都是 SSL 直连，587 的 STARTTLS 反而常被封。
	defaultMailSSL = true
)

// MailConfig 邮件配置，对应 yaml 的 mail 段。
//
// 只为发验证码而存在，故不含附件、HTML、多收件人等能力。
type MailConfig struct {
	// Enabled 是否开启邮件功能。关闭时发信接口返回提示而非报错。
	Enabled bool `mapstructure:"enabled"`
	// Host SMTP 服务器地址，如 smtp.qq.com。
	Host string `mapstructure:"host"`
	// Port SMTP 端口：465 配 ssl=true，587 配 ssl=false（STARTTLS）。
	Port int `mapstructure:"port"`
	// From 发件地址，同时用作信封发件人。
	From string `mapstructure:"from"`
	// User SMTP 登录账号，多数服务商即完整邮箱地址。
	User string `mapstructure:"user"`
	// Password SMTP 登录密码。QQ/163 等要填「授权码」而非登录密码。
	Password string `mapstructure:"password"`
	// SSL true 走 SSL 直连（465），false 走 STARTTLS 升级（587）。
	SSL bool `mapstructure:"ssl"`
}

// DefaultMail 返回默认配置。默认关闭——没配 SMTP 却开着，只会让接口一直报错。
func DefaultMail() MailConfig {
	return MailConfig{
		Enabled: false,
		Port:    defaultMailPort,
		SSL:     defaultMailSSL,
	}
}

// setDefaults 把默认值铺给 viper。
func (c MailConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("mail.enabled", c.Enabled)
	v.SetDefault("mail.host", c.Host)
	v.SetDefault("mail.port", c.Port)
	v.SetDefault("mail.from", c.From)
	v.SetDefault("mail.user", c.User)
	v.SetDefault("mail.password", c.Password)
	v.SetDefault("mail.ssl", c.SSL)
}

// validate 校验邮件配置。
func (c MailConfig) validate() error {
	// 关闭时其余项无需校验（同 push）：没开邮件功能不该拦住进程启动。
	if !c.Enabled {
		return nil
	}
	if c.Host == "" {
		return errMissing("mail.host")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errInvalid("mail.port", "必须是 1-65535 的端口号")
	}
	if c.From == "" {
		return errMissing("mail.from")
	}
	if c.User == "" {
		return errMissing("mail.user")
	}
	if c.Password == "" {
		return errMissing("mail.password")
	}
	return nil
}
