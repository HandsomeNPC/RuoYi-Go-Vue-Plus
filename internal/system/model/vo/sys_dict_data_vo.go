package vo

import (
	"time"
)

// SysDictDataVo 字典数据视图对象，对应 Java SysDictDataVo。
type SysDictDataVo struct {
	DictCode  int64  `json:"dictCode"`
	DictSort  int    `json:"dictSort"`
	DictLabel string `json:"dictLabel"`
	DictValue string `json:"dictValue"`
	DictType  string `json:"dictType"`
	CssClass  string `json:"cssClass"`
	ListClass string `json:"listClass"`
	// IsDefault 是否默认（Y是 N否）。
	IsDefault  string     `json:"isDefault"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
}
