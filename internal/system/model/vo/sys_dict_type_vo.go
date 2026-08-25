package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysDictTypeVo 字典类型视图对象，对应 Java SysDictTypeVo。
type SysDictTypeVo struct {
	DictID     int64      `json:"dictId"`
	DictName   string     `json:"dictName"`
	DictType   string     `json:"dictType"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
}

// FromSysDictType 把实体转成 VO。
func FromSysDictType(t *systemmodel.SysDictType) *SysDictTypeVo {
	if t == nil {
		return nil
	}
	return &SysDictTypeVo{
		DictID:     t.DictID,
		DictName:   t.DictName,
		DictType:   t.DictType,
		Remark:     t.Remark,
		CreateTime: t.CreateTime,
	}
}
