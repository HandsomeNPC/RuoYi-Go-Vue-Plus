package bo

// SysOssBo OSS 对象存储业务对象（入参），对应 Java SysOssBo。
//
// 查询条件另见 SysOssQueryBo：Java 侧 list 复用本型 + QueryGroup，
// 但 Go 的 binding tag 没有校验分组，两种场景只能分开建型。
type SysOssBo struct {
	OssID        int64  `json:"ossId"`
	FileName     string `json:"fileName"`
	OriginalName string `json:"originalName"`
	FileSuffix   string `json:"fileSuffix"`
	URL          string `json:"url"`
	Ext1         string `json:"ext1"`
	Service      string `json:"service"`
	CreateBy     int64  `json:"createBy"`
}
