package config

import (
	"github.com/spf13/viper"

	"ruoyi-go-vue-plus/pkg/encrypt"
)

// DefaultAPIEncryptHeader 传递 AES 密钥的头名。
const DefaultAPIEncryptHeader = "encrypt-key"

// APIEncrypt 接口加解密配置。
type APIEncrypt struct {
	// Enabled 是否启用接口加解密。
	Enabled bool `mapstructure:"enabled"`

	// HeaderFlag 传递 AES 密钥的请求/响应头名，为空表示用 DefaultAPIEncryptHeader。
	HeaderFlag string `mapstructure:"headerFlag"`

	// PublicKey 响应加密用的 RSA 公钥，base64 编码的 X.509 SPKI。
	PublicKey string `mapstructure:"publicKey"`

	// PrivateKey 请求解密用的 RSA 私钥，base64 编码的 PKCS#8。
	PrivateKey string `mapstructure:"privateKey"`

	// RequestURLs 必须加密的接口路径，Ant 风格。
	RequestURLs []string `mapstructure:"requestUrls"`

	// ResponseURLs 需要加密响应体的接口路径，Ant 风格。
	ResponseURLs []string `mapstructure:"responseUrls"`

	// MaxBodySize 允许读取的最大密文字节数，超出则拒绝请求。<=0 表示用默认值。
	MaxBodySize int64 `mapstructure:"maxBodySize"`
}

// defaultAPIEncrypt 返回默认配置。
func defaultAPIEncrypt() APIEncrypt {
	return APIEncrypt{
		Enabled:     false,
		HeaderFlag:  DefaultAPIEncryptHeader,
		MaxBodySize: defaultMaxBodySize,
	}
}

// setDefaults 把默认值铺给 viper。
func (a APIEncrypt) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.apiEncrypt.enabled", a.Enabled)
	v.SetDefault("middleware.apiEncrypt.headerFlag", a.HeaderFlag)
	v.SetDefault("middleware.apiEncrypt.publicKey", a.PublicKey)
	v.SetDefault("middleware.apiEncrypt.privateKey", a.PrivateKey)
	v.SetDefault("middleware.apiEncrypt.requestUrls", a.RequestURLs)
	v.SetDefault("middleware.apiEncrypt.responseUrls", a.ResponseURLs)
	v.SetDefault("middleware.apiEncrypt.maxBodySize", a.MaxBodySize)
}

// validate 校验接口加解密配置。
func (a APIEncrypt) validate() error {
	if a.MaxBodySize < 0 {
		return errInvalid("middleware.apiEncrypt.maxBodySize", "不能为负数")
	}

	if !a.Enabled {
		return nil
	}

	if a.PrivateKey == "" {
		return errMissing("middleware.apiEncrypt.privateKey")
	}
	if _, err := encrypt.ParseRSAPrivateKey(a.PrivateKey); err != nil {
		return errInvalid("middleware.apiEncrypt.privateKey", err.Error())
	}

	if len(a.ResponseURLs) == 0 {
		return nil
	}
	if a.PublicKey == "" {
		return errInvalid("middleware.apiEncrypt.publicKey",
			"配置了 responseUrls 时必填（响应加密需要它加密 AES 秘钥）")
	}
	if _, err := encrypt.ParseRSAPublicKey(a.PublicKey); err != nil {
		return errInvalid("middleware.apiEncrypt.publicKey", err.Error())
	}
	return nil
}
