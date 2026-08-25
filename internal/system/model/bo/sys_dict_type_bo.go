package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysDictTypeBo 字典类型业务对象（入参），对应 Java SysDictTypeBo。
type SysDictTypeBo struct {
	DictID   int64  `json:"dictId"`
	DictName string `json:"dictName" binding:"required,max=100"`
	DictType string `json:"dictType" binding:"required,max=100"`
	Remark   string `json:"remark"`
}

// ToSysDictType 把 BO 转成实体。
func (b *SysDictTypeBo) ToSysDictType() *systemmodel.SysDictType {
	if b == nil {
		return nil
	}
	return &systemmodel.SysDictType{
		DictID:   b.DictID,
		DictName: b.DictName,
		DictType: b.DictType,
		Remark:   b.Remark,
	}
}
