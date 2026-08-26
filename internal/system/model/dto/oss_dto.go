package dto

// OssDTO OSS 文件简要信息对象，对应 Java org.dromara.system.api.domain.OssDTO。
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
