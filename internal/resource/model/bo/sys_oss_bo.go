package bo

// SysOssBo OSS 对象存储业务对象（入参）。
//
// 查询条件另见 SysOssQueryBo：Go 的 binding tag 没有校验分组，
// 写入与查询两种场景只能分开建型。
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
