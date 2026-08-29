package config

import "github.com/spf13/viper"

// 验证码类型常量。
const (
	// CaptchaTypeMath 算术验证码，如 3 + 5 = ? 8。
	CaptchaTypeMath = "math"
	// CaptchaTypeChar 字符验证码，随机取 charLength 个字符。
	CaptchaTypeChar = "char"
)

// defaultCaptchaType 默认验证码类型。
const defaultCaptchaType = CaptchaTypeMath

// CaptchaConfig 验证码配置，对应 yaml 的 captcha 段。
//
// 仅暴露 4 个可配项(启用开关、类型、算术位数、字符长度)；图片渲染、
// 干扰线、字体等在 captcha service 里固定写死，与原项目对齐。
type CaptchaConfig struct {
	// Enable 是否启用验证码校验。
	Enable bool `mapstructure:"enable"`
	// Type 验证码类型：math 算术计算 / char 字符验证。
	Type string `mapstructure:"type"`
	// NumberLength 算术验证码中每个操作数的位数，如 1 生成个位、2 生成两位。
	NumberLength int `mapstructure:"numberLength"`
	// CharLength 字符验证码的长度(字符个数)。
	CharLength int `mapstructure:"charLength"`
}

// DefaultCaptcha 返回默认配置。
func DefaultCaptcha() CaptchaConfig {
	return CaptchaConfig{
		Enable:       false,
		Type:         defaultCaptchaType,
		NumberLength: 1,
		CharLength:   4,
	}
}

// setDefaults 把默认值铺给 viper。
func (c CaptchaConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("captcha.enable", c.Enable)
	v.SetDefault("captcha.type", c.Type)
	v.SetDefault("captcha.numberLength", c.NumberLength)
	v.SetDefault("captcha.charLength", c.CharLength)
}

// validate 校验验证码配置。
func (c CaptchaConfig) validate() error {
	// 关闭时其余项无需校验。
	if !c.Enable {
		return nil
	}
	switch c.Type {
	case CaptchaTypeMath, CaptchaTypeChar:
	default:
		return errInvalid("captcha.type", "只能是 math 或 char")
	}
	if c.NumberLength <= 0 {
		return errInvalid("captcha.numberLength", "必须为正整数")
	}
	if c.CharLength <= 0 {
		return errInvalid("captcha.charLength", "必须为正整数")
	}
	return nil
}
