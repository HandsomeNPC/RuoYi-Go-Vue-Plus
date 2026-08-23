package config

import (
	"github.com/spf13/viper"

	"ruoyi-go-vue-plus/pkg/encrypt"
)

// DefaultAPIEncryptHeader 传递 AES 密钥的头名，对应
// ApiDecryptProperties.headerFlag 的默认值 encrypt-key（application.yml:151）。
//
// 这个头在**请求与响应两个方向上都用同一个名字**，但载荷不同：
// 请求方向是「前端用服务端公钥加密的 AES 密钥」，响应方向是「服务端用
// 前端公钥加密的 AES 密钥」。共用一个名字是原项目的协议约定，不能改 ——
// 前端两侧读写的都是它。
const DefaultAPIEncryptHeader = "encrypt-key"

// APIEncrypt 接口加解密配置，对应 properties/ApiDecryptProperties.java（前缀 api-decrypt）。
//
// # 与其余中间件不同，本项目**有** Enabled 开关
//
// Middleware 的说明里写了「有意没有 enabled 开关，注册即启用」。这一项是
// 例外，理由是它与其余中间件的失败方向相反：
//
//   - XSS / AccessLog 这类不注册就是少一层清洗或少几行日志，请求照常处理。
//   - 本中间件不注册，则带 encrypt-key 头的请求会被**当作明文**交给 handler，
//     JSON 解析必然失败，前端收到的是一句莫名的参数错误 —— 而真正的原因
//     （服务端没开解密）在报文里没有任何痕迹。
//
// 也就是说这里必须能区分「没开」与「开了但密钥配错」，Enabled 就是那个区分。
// 对齐 Java 侧的 @ConditionalOnProperty(api-decrypt.enabled)，
// 且原项目 application.yml:150 该值为 **true**。
type APIEncrypt struct {
	// Enabled 是否启用接口加解密。
	//
	// 关闭时 middleware.APIEncrypt() 返回一个空操作中间件，
	// 而不是在 Register 里跳过 —— 这样「关闭」与「启用但无匹配请求」
	// 走的是同一条代码路径，少一种只在特定配置下才跑到的分支。
	Enabled bool `mapstructure:"enabled"`

	// HeaderFlag 传递 AES 密钥的请求/响应头名，为空表示用 DefaultAPIEncryptHeader。
	HeaderFlag string `mapstructure:"headerFlag"`

	// PublicKey 响应加密用的 RSA 公钥，base64 编码的 X.509 SPKI。
	//
	// 对应前端的解密私钥。只有 ResponseURLs 命中的接口才会用到它，
	// 所以那个列表为空时它可以留空（validate 据此放行）。
	PublicKey string `mapstructure:"publicKey"`

	// PrivateKey 请求解密用的 RSA 私钥，base64 编码的 PKCS#8。
	//
	// 对应前端的加密公钥。Enabled 为 true 时必填 —— 没有它就解不了任何请求，
	// 而那种「开了但每个加密请求都失败」的状态应该在启动期暴露。
	PrivateKey string `mapstructure:"privateKey"`

	// RequestURLs 必须加密的接口路径，Ant 风格（见 pkg/middleware/path.go）。
	//
	// 对应 Java 侧标了 @ApiEncrypt 的方法 —— 那边靠
	// RequestMappingHandlerMapping 反查注解，Go 无注解，改为显式路径清单。
	//
	// 语义是**强制**：命中的路径若没带 encrypt-key 头，请求被拒
	// （对齐 CryptoFilter 里「有注解却没有加密标头就报 403」的分支）。
	// 未命中的路径带了头则照常解密 —— 与 Java 一致，那边解密只看头、不看注解。
	RequestURLs []string `mapstructure:"requestUrls"`

	// ResponseURLs 需要加密响应体的接口路径，Ant 风格。
	//
	// 对应 @ApiEncrypt(response = true)。**原项目 4 处 @ApiEncrypt 全部是
	// 默认的 response = false**，即这条链路在原项目里从未启用过，
	// 故默认为空。开启前请先确认前端确实实现了响应解密。
	ResponseURLs []string `mapstructure:"responseUrls"`

	// MaxBodySize 允许读取的最大密文字节数，超出则拒绝请求。<=0 表示用默认值。
	//
	// 与 RepeatableBody.MaxBodySize 是两个独立的项，尽管默认值相同：
	// 解密发生在缓存**之前**，读的是密文，而 base64 密文比明文大约 4/3 ——
	// 真要卡紧上限时，两处该配的数不一样。
	//
	// Java 侧无此上限（DecryptRequestBodyWrapper 用 IoUtil.readBytes 读到底），
	// 理由与 body.go 那条相同：无上限的 io.ReadAll 等于让调用方决定进程吃多少内存。
	MaxBodySize int64 `mapstructure:"maxBodySize"`
}

// defaultAPIEncrypt 返回默认配置。
//
// **Enabled 默认为 false，这是相对原项目的有意偏差**（那边 yaml 里是 true）。
// 理由：默认值要在「没有配置文件」时也讲得通，而启用状态下缺私钥必须报错，
// 于是默认 true + 默认空私钥这个组合会让任何未配置的进程启动失败。
// 仓库的 configs/application.yaml 里显式配成了 true 并带上密钥，
// 与原项目对齐的是那份文件。
func defaultAPIEncrypt() APIEncrypt {
	return APIEncrypt{
		Enabled:     false,
		HeaderFlag:  DefaultAPIEncryptHeader,
		MaxBodySize: defaultMaxBodySize,
	}
}

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
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
//
// 关闭时完全不校验：密钥留空是关闭状态下的正常形态，此时报错会逼着
// 每个不用这个功能的部署都去填一对无用的密钥。
//
// 启用时**在启动期就把密钥解析一遍**，对齐 CryptoFilter 构造函数里那两行
// validateRsaPublicKey / validateRsaPrivateKey。这是刻意的：密钥格式错误
// 若留到运行期，表现是「所有加密接口都失败」而其余接口正常，
// 那种半死状态比启动失败难查得多。
func (a APIEncrypt) validate() error {
	// 这一项与开关无关：负数在任何状态下都讲不通，
	// 放过它只会让「关闭时配错、将来开启才炸」成为可能。
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

	// 公钥只在响应加密时用得到，没配 responseUrls 就不强求 ——
	// 这是本项目的常态（原项目从未启用响应加密）。
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
