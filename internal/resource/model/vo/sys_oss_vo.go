package vo

import (
	"time"
)

// SysOssVo OSS 对象存储视图对象，对应 Java SysOssVo。
//
// 无 excel tag：Java 的 SysOssVo 未标 @ExcelProperty，两侧均无导出接口。
type SysOssVo struct {
	OssID        int64      `json:"ossId"`
	FileName     string     `json:"fileName"`
	OriginalName string     `json:"originalName"`
	FileSuffix   string     `json:"fileSuffix"`
	URL          string     `json:"url"`
	Ext1         string     `json:"ext1"`
	CreateTime   *time.Time `json:"createTime"`
	CreateBy     int64      `json:"createBy"`
	// CreateByName 上传人名称，由翻译层按 USER_ID_TO_NAME 从 CreateBy 回填。
	CreateByName string `json:"createByName"`
	Service      string `json:"service"`
}
