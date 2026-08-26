package bo

// SysOssBo OSS 对象存储业务对象（入参），对应 Java SysOssBo。
type SysOssBo struct {
	OssID        int64  `json:"ossId"`
	FileName     string `json:"fileName"`
	OriginalName string `json:"originalName"`
	FileSuffix   string `json:"fileSuffix"`
	URL          string `json:"url"`
	Ext1         string `json:"ext1"`
	Service      string `json:"service"`
	CreateBy     int64  `json:"createBy"`
	// Params 请求参数袋，不落表。
	Params map[string]any `json:"params"`
}
