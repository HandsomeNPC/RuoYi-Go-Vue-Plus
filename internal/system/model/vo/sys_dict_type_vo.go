package vo

import (
	"time"
)

// SysDictTypeVo 字典类型视图对象，对应 Java SysDictTypeVo。
type SysDictTypeVo struct {
	DictID     int64      `json:"dictId" excel:"字典主键"`
	DictName   string     `json:"dictName" excel:"字典名称"`
	DictType   string     `json:"dictType" excel:"字典类型"`
	Remark     string     `json:"remark" excel:"备注"`
	CreateTime *time.Time `json:"createTime" excel:"创建时间"`
}
