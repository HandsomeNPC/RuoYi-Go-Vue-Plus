package model

import (
	coremodel "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model"
)

// SysDictType 字典类型表（sys_dict_type）。
type SysDictType struct {
	DictID   int64  `gorm:"column:dict_id;primaryKey" json:"dictId"`
	DictName string `gorm:"column:dict_name" json:"dictName"`
	DictType string `gorm:"column:dict_type" json:"dictType"`
	Remark   string `gorm:"column:remark" json:"remark"`

	coremodel.BaseEntity
}

// TableName 显式指定表名。
func (SysDictType) TableName() string {
	return "sys_dict_type"
}
