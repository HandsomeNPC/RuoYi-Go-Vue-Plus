package vo

import systemmodel "ruoyi-go-vue-plus/internal/system/model"

// SysOssConfigVo 对象存储配置视图对象，对应 Java SysOssConfigVo。
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
	// AccessPolicy 桶权限类型（0private 1public 2custom）。
	AccessPolicy string `json:"accessPolicy"`
}

// FromSysOssConfig 把实体转成 VO。
func FromSysOssConfig(c *systemmodel.SysOssConfig) *SysOssConfigVo {
	if c == nil {
		return nil
	}
	return &SysOssConfigVo{
		OssConfigID:  c.OssConfigID,
		ConfigKey:    c.ConfigKey,
		AccessKey:    c.AccessKey,
		SecretKey:    c.SecretKey,
		BucketName:   c.BucketName,
		Prefix:       c.Prefix,
		Endpoint:     c.Endpoint,
		DomainURL:    c.DomainURL,
		IsHttps:      c.IsHttps,
		Region:       c.Region,
		Status:       c.Status,
		Ext1:         c.Ext1,
		Remark:       c.Remark,
		AccessPolicy: c.AccessPolicy,
	}
}
