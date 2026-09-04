package model

// SysOssExt 附件扩展字段对象。
//
// 不映射表，序列化为 JSON 存入 sys_oss.ext1 列，故无 gorm 列标签与 TableName。
type SysOssExt struct {
	// BizType 所属业务类型（如 avatar、report、contract）。
	BizType string `json:"bizType"`
	// FileSize 文件大小（字节）。
	FileSize int64 `json:"fileSize"`
	// ContentType 文件类型（MIME，如 image/png）。
	ContentType string `json:"contentType"`
	// Source 来源标识（如 userUpload、systemImport）。
	Source string `json:"source"`
	// UploadIP 上传 IP 地址，便于审计追踪。
	UploadIP string `json:"uploadIp"`
	// Remark 附件说明或备注。
	Remark string `json:"remark"`
	// Tags 附件标签，如 ["图片","证件"]。
	Tags []string `json:"tags"`
	// RefID 业务绑定ID。
	RefID string `json:"refId"`
	// RefType 绑定业务类型。
	RefType string `json:"refType"`
	// IsTemp 是否为临时文件，用于区分正式或待清理。
	IsTemp bool `json:"isTemp"`
	// MD5 文件 MD5，可用于去重或校验。
	MD5 string `json:"md5"`
}
