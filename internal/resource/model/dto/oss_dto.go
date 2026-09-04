package dto

// OssDTO OSS 文件简要信息对象。
// 供跨模块取文件 URL（如用户头像）用，不含审计字段。
type OssDTO struct {
	// OssID 对象存储主键。
	OssID int64 `json:"ossId"`
	// FileName 文件名。
	FileName string `json:"fileName"`
	// OriginalName 原名。
	OriginalName string `json:"originalName"`
	// FileSuffix 文件后缀名。
	FileSuffix string `json:"fileSuffix"`
	// URL URL地址。
	URL string `json:"url"`
}
