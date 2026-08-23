package config

import "github.com/spf13/viper"

// ContentTypeJSON JSON 请求的 content-type 前缀。
//
// 用前缀匹配而非全等，因为实际请求多半带参数（application/json;charset=UTF-8），
// 对齐 RepeatableFilter 里的 startsWithIgnoreCase(contentType, APPLICATION_JSON_VALUE)。
const ContentTypeJSON = "application/json"

// defaultMaxBodySize 默认缓存上限 10MB，与 Java 侧 spring.servlet.multipart.max-file-size
// 取同一个数量级（application.yml:70）。
//
// 原项目这里**没有上限**：RepeatedlyRequestWrapper 用 IoUtil.readBytes 一次读完，
// 而 Tomcat 的 max-http-form-post-size 只管表单、不管 JSON。Java 侧能这么写是因为
// 它前面有 nginx 的 client_max_body_size 兜着；Go 侧仍然要自己设限 ——
// 无上限的 io.ReadAll 等于让调用方决定进程吃多少内存，一个几 GB 的 chunked
// 请求就能把进程 OOM 掉，这是纯粹的放大攻击（代价在服务端，成本在客户端）。
const defaultMaxBodySize = 10 << 20

// RepeatableBody 可重复读 body 的配置。
type RepeatableBody struct {
	// ContentTypes 需要缓存的 content-type 前缀（小写），大小写不敏感匹配。
	//
	// 默认只含 application/json，与 RepeatableFilter 一致。**不要**为了让日志
	// 打得更全而把 multipart/form-data 加进来：那会把上传的文件整个读进内存，
	// 原项目允许 10MB 单文件 / 20MB 单请求，并发几个就够把进程压垮。
	//
	// 表单请求（application/x-www-form-urlencoded）也不需要加：
	// net/http 的 ParseForm 会把解析结果缓存进 r.PostForm，
	// 后续中间件与 handler 读的是解析结果而不是 body，天然可重复。
	// 这与 Java 侧 AccessLog 走 getParameterMap() 而非读 body 是同一个道理。
	ContentTypes []string `mapstructure:"contentTypes"`

	// MaxBodySize 允许缓存的最大字节数，超出则拒绝请求。<=0 表示用默认值。
	MaxBodySize int64 `mapstructure:"maxBodySize"`
}

// defaultRepeatableBody 返回默认配置。
func defaultRepeatableBody() RepeatableBody {
	return RepeatableBody{
		ContentTypes: []string{ContentTypeJSON},
		MaxBodySize:  defaultMaxBodySize,
	}
}

// setDefaults 把默认值铺给 viper，键名与 mapstructure tag 一一对应。
func (r RepeatableBody) setDefaults(v *viper.Viper) {
	v.SetDefault("middleware.repeatableBody.contentTypes", r.ContentTypes)
	v.SetDefault("middleware.repeatableBody.maxBodySize", r.MaxBodySize)
}

// validate 校验可重复读 body 配置。
func (r RepeatableBody) validate() error {
	if r.MaxBodySize < 0 {
		return errInvalid("middleware.repeatableBody.maxBodySize", "不能为负数")
	}
	return nil
}
