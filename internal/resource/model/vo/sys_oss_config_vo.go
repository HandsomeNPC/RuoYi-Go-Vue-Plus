package vo

// SysOssConfigVo 对象存储配置视图对象。
type SysOssConfigVo struct {
	OssConfigID int64  `json:"ossConfigId"`
	ConfigKey   string `json:"configKey"`
	AccessKey   string `json:"accessKey"`
	SecretKey   string `json:"secretKey"`
	BucketName  string `json:"bucketName"`
	Prefix      string `json:"prefix"`
	Endpoint    string `json:"endpoint"`
	DomainURL   string `json:"domainUrl"`
	// IsHttps 是否 https（Y是 N否）。
	IsHttps string `json:"isHttps"`
	Region  string `json:"region"`
	// Status 是否默认（Y是 N否）。
	Status string `json:"status"`
	Ext1   string `json:"ext1"`
	Remark string `json:"remark"`
	// AccessPolicy 桶权限类型（0private 1public-read-write 2public-read）。
	AccessPolicy string `json:"accessPolicy"`
}
