package model

// SysOssConfig 对象存储配置表（sys_oss_config），对应 Java org.dromara.system.domain.SysOssConfig。
type SysOssConfig struct {
	OssConfigID int64  `gorm:"column:oss_config_id;primaryKey" json:"ossConfigId"`
	ConfigKey   string `gorm:"column:config_key" json:"configKey"`
	AccessKey   string `gorm:"column:access_key" json:"accessKey"`
	SecretKey   string `gorm:"column:secret_key" json:"secretKey"`
	BucketName  string `gorm:"column:bucket_name" json:"bucketName"`
	Prefix      string `gorm:"column:prefix" json:"prefix"`
	Endpoint    string `gorm:"column:endpoint" json:"endpoint"`
	DomainURL   string `gorm:"column:domain_url" json:"domainUrl"`
	// IsHttps 是否https（0否 1是）。
	IsHttps string `gorm:"column:is_https" json:"isHttps"`
	Region  string `gorm:"column:region" json:"region"`
	// Status 是否默认（Y是 N否）。
	Status string `gorm:"column:status" json:"status"`
	Ext1   string `gorm:"column:ext1" json:"ext1"`
	Remark string `gorm:"column:remark" json:"remark"`
	// AccessPolicy 桶权限类型（0private 1public 2custom）。
	AccessPolicy string `gorm:"column:access_policy" json:"accessPolicy"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysOssConfig) TableName() string {
	return "sys_oss_config"
}
