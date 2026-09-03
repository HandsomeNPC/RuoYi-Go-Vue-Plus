package bo

// SysOssConfigBo 对象存储配置业务对象（入参），对应 Java SysOssConfigBo。
type SysOssConfigBo struct {
	OssConfigID int64  `json:"ossConfigId"`
	ConfigKey   string `json:"configKey" binding:"required,min=2,max=100"`
	AccessKey   string `json:"accessKey" binding:"required,min=2,max=100"`
	SecretKey   string `json:"secretKey" binding:"required,min=2,max=100"`
	BucketName  string `json:"bucketName" binding:"required,min=2,max=100"`
	Prefix      string `json:"prefix"`
	Endpoint    string `json:"endpoint" binding:"required,min=2,max=100"`
	DomainURL   string `json:"domainUrl"`
	// IsHttps 是否 https（Y是 N否）。
	IsHttps string `json:"isHttps"`
	// Status 是否默认（Y是 N否）。
	Status string `json:"status"`
	Region string `json:"region"`
	Ext1   string `json:"ext1"`
	Remark string `json:"remark"`
	// AccessPolicy 桶权限类型（0private 1public-read-write 2public-read）。
	AccessPolicy string `json:"accessPolicy" binding:"required"`
}
