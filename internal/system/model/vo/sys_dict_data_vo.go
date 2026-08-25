package vo

import (
	"time"

	systemmodel "ruoyi-go-vue-plus/internal/system/model"
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

// FromSysDictData 把实体转成 VO。
func FromSysDictData(d *systemmodel.SysDictData) *SysDictDataVo {
	if d == nil {
		return nil
	}
	return &SysDictDataVo{
		DictCode:   d.DictCode,
		DictSort:   d.DictSort,
		DictLabel:  d.DictLabel,
		DictValue:  d.DictValue,
		DictType:   d.DictType,
		CssClass:   d.CssClass,
		ListClass:  d.ListClass,
		IsDefault:  d.IsDefault,
		Remark:     d.Remark,
		CreateTime: d.CreateTime,
	}
}
