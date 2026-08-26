package vo

import (
	"time"
)

// SysDictTypeVo 字典类型视图对象，对应 Java SysDictTypeVo。
type SysDictTypeVo struct {
	DictID     int64      `json:"dictId"`
	DictName   string     `json:"dictName"`
	DictType   string     `json:"dictType"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
}
