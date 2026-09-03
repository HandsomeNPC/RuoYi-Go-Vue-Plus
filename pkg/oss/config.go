// Package oss 对象存储客户端，移植自 Java ruoyi-common-oss。
//
// 底层统一走 S3 协议（aws-sdk-go-v2），厂商差异只体现在 endpoint 与寻址风格上，
// 不为 minio/阿里云/腾讯云各引一套 SDK。
package oss

import "strings"

// 桶权限类型，对应 Java AccessPolicy 枚举的 type 值。
//
// 建表 SQL 把 2 注释成 custom，与枚举不符——以枚举为准（Java 侧 AccessPolicy.formType
// 按这三个值匹配，写 custom 会直接抛异常）。
const (
	// AccessPolicyPrivate 私有桶。仅此值会让访问地址换成预签名链接。
	AccessPolicyPrivate = "0"
	// AccessPolicyPublicReadWrite 公共读写。
	AccessPolicyPublicReadWrite = "1"
	// AccessPolicyPublicRead 公共读。
	AccessPolicyPublicRead = "2"
)

// defaultRegion 未配置 region 时的兜底，对齐 Java OssClientConfig.parseRegion。
const defaultRegion = "us-east-1"

// cloudServices 走虚拟主机寻址（bucket.endpoint）的厂商标识。
//
// 判定方式是 endpoint 子串匹配，对齐 Java resolvePathStyleAccess：
// 命中即虚拟主机风格，否则路径风格。MinIO 等自建服务不含这些串，故走路径风格
// ——它们多数不支持虚拟主机寻址，用错风格会直接连不上。
var cloudServices = []string{"aliyun", "qcloud", "qiniu", "obs"}

// Properties 对象存储配置的反序列化目标，字段对应 sys_oss_config 的列。
//
// 缓存里存的是完整 SysOssConfig 的 JSON，多出来的列（configKey/status/审计字段）
// 由 json 解码静默忽略，与 Java OssProperties 的处理一致。
type Properties struct {
	Endpoint     string `json:"endpoint"`
	DomainURL    string `json:"domainUrl"`
	Prefix       string `json:"prefix"`
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	BucketName   string `json:"bucketName"`
	Region       string `json:"region"`
	IsHttps      string `json:"isHttps"`
	AccessPolicy string `json:"accessPolicy"`
}

// ClientConfig 由 Properties 推导出的客户端配置。
//
// 全部字段可比较：工厂据此判断已缓存的客户端是否仍与最新配置一致
// （对齐 Java OssClientConfig 的 @EqualsAndHashCode + verifyConfig）。
type ClientConfig struct {
	Endpoint  string
	Domain    string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	Prefix    string
	// UseHTTPS 由 isHttps == "Y" 推出。
	UseHTTPS bool
	// UsePathStyle 路径寻址（endpoint/bucket）还是虚拟主机寻址（bucket.endpoint）。
	UsePathStyle bool
	AccessPolicy string
}

// NewClientConfig 按 Java OssClientConfig.formPropertiesBuilder 的口径推导配置。
func NewClientConfig(p Properties) ClientConfig {
	region := strings.TrimSpace(p.Region)
	if region == "" {
		region = defaultRegion
	}
	accessPolicy := strings.TrimSpace(p.AccessPolicy)
	if accessPolicy == "" {
		accessPolicy = AccessPolicyPublicReadWrite
	}

	return ClientConfig{
		Endpoint:     strings.TrimSpace(p.Endpoint),
		Domain:       strings.TrimSpace(p.DomainURL),
		AccessKey:    p.AccessKey,
		SecretKey:    p.SecretKey,
		Bucket:       strings.TrimSpace(p.BucketName),
		Region:       region,
		Prefix:       strings.TrimSpace(p.Prefix),
		UseHTTPS:     p.IsHttps == "Y",
		UsePathStyle: !isCloudService(p.Endpoint),
		AccessPolicy: accessPolicy,
	}
}

// isCloudService 判断 endpoint 是否属于支持虚拟主机寻址的公有云。
func isCloudService(endpoint string) bool {
	lower := strings.ToLower(endpoint)
	for _, s := range cloudServices {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// IsPrivate 是否私有桶。仅私有桶需要把访问地址换成预签名链接。
func (c ClientConfig) IsPrivate() bool {
	return c.AccessPolicy == AccessPolicyPrivate
}

// EndpointURL 带协议头的服务地址，供 S3 客户端做 endpoint 覆盖。
func (c ClientConfig) EndpointURL() string {
	return rebuildURLHeader(c.UseHTTPS, c.Endpoint)
}

// DomainURL 带协议头的自定义域名，未配置时回落服务地址。
// 预签名走这里而非 EndpointURL，使签出的链接指向 CDN（对齐 Java 的 presigner endpoint）。
func (c ClientConfig) DomainURL() string {
	if c.Domain != "" {
		return rebuildURLHeader(c.UseHTTPS, c.Domain)
	}
	return c.EndpointURL()
}

// BucketURL 桶的访问前缀，落库的文件 URL 即 BucketURL + "/" + key。
func (c ClientConfig) BucketURL() string {
	base := c.Domain
	if base == "" {
		base = c.Endpoint
	}
	if c.UsePathStyle {
		return pathStyleBucketURL(c.UseHTTPS, base, c.Bucket)
	}
	return siteStyleBucketURL(c.UseHTTPS, base, c.Bucket)
}
