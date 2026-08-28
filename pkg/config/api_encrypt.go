package config

import "github.com/spf13/viper"

// DefaultAPIEncryptHeader 传递 AES 密钥的头名。
const DefaultAPIEncryptHeader = "encrypt-key"

// APIEncryptConfig 接口加解密配置
type APIEncryptConfig struct {
	// Enabled 是否启用。false 时 encrypt.Init 返回 no-op，所有注解直通。
	Enabled bool `mapstructure:"enabled"`
	// HeaderFlag 传递 AES 密钥的请求/响应头名，为空表示用 DefaultAPIEncryptHeader。
	HeaderFlag string `mapstructure:"headerFlag"`
	// PublicKey 响应加密用的 RSA 公钥，base64 编码的 X.509 SPKI。
	PublicKey string `mapstructure:"publicKey"`
	// PrivateKey 请求解密用的 RSA 私钥，base64 编码的 PKCS#8。
	PrivateKey string `mapstructure:"privateKey"`
}

// DefaultAPIEncrypt 返回接口加解密默认配置。
func DefaultAPIEncrypt() APIEncryptConfig {
	return APIEncryptConfig{
		Enabled:    false,
		HeaderFlag: DefaultAPIEncryptHeader,
	}
}

// setDefaults 把默认值铺给 viper。
func (a APIEncryptConfig) setDefaults(v *viper.Viper) {
	v.SetDefault("apiEncrypt.enabled", a.Enabled)
	v.SetDefault("apiEncrypt.headerFlag", a.HeaderFlag)
	v.SetDefault("apiEncrypt.publicKey", a.PublicKey)
	v.SetDefault("apiEncrypt.privateKey", a.PrivateKey)
}

// validate 校验接口加解密配置
func (a APIEncryptConfig) validate() error {
	if !a.Enabled {
		return nil
	}
	if a.PrivateKey == "" {
		return errMissing("apiEncrypt.privateKey")
	}
	return nil
}
