package model

// SysOss OSS 对象存储表（sys_oss），对应 Java org.dromara.system.domain.SysOss。
// ext1 列存 SysOssExt 序列化后的 JSON（见 sys_oss_ext.go）。
type SysOss struct {
	OssID        int64  `gorm:"column:oss_id;primaryKey" json:"ossId"`
	FileName     string `gorm:"column:file_name" json:"fileName"`
	OriginalName string `gorm:"column:original_name" json:"originalName"`
	FileSuffix   string `gorm:"column:file_suffix" json:"fileSuffix"`
	URL          string `gorm:"column:url" json:"url"`
	Ext1         string `gorm:"column:ext1" json:"ext1"`
	Service      string `gorm:"column:service" json:"service"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysOss) TableName() string {
	return "sys_oss"
}
