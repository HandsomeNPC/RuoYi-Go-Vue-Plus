package model

// SysDictType 字典类型表（sys_dict_type），对应 Java org.dromara.system.domain.SysDictType。
type SysDictType struct {
	DictID   int64  `gorm:"column:dict_id;primaryKey" json:"dictId"`
	DictName string `gorm:"column:dict_name" json:"dictName"`
	DictType string `gorm:"column:dict_type" json:"dictType"`
	Remark   string `gorm:"column:remark" json:"remark"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysDictType) TableName() string {
	return "sys_dict_type"
}
