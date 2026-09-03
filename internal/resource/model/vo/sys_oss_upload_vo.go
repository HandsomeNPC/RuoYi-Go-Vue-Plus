package vo

// SysOssUploadVo 上传结果，对应 Java SysOssController.SysOssUploadVo record。
type SysOssUploadVo struct {
	// URL 文件访问地址，私有桶时为预签名链接。
	URL string `json:"url"`
	// FileName 原始文件名（非对象 key，对齐 Java 取 originalName）。
	FileName string `json:"fileName"`
	// OssID 对象存储主键。
	//
	// 裸 int64 而非 Java 的字符串：pkg/jsonx 已接管 gin codec 按值转字符串，
	// 加 ,string tag 反而会拒收数字形态。
	OssID int64 `json:"ossId"`
}
