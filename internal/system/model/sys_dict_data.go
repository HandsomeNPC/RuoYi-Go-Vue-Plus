package model

// SysDictData 字典数据表（sys_dict_data），对应 Java org.dromara.system.domain.SysDictData。
type SysDictData struct {
	DictCode  int64  `gorm:"column:dict_code;primaryKey" json:"dictCode"`
	DictSort  int    `gorm:"column:dict_sort" json:"dictSort"`
	DictLabel string `gorm:"column:dict_label" json:"dictLabel"`
	DictValue string `gorm:"column:dict_value" json:"dictValue"`
	DictType  string `gorm:"column:dict_type" json:"dictType"`
	CssClass  string `gorm:"column:css_class" json:"cssClass"`
	ListClass string `gorm:"column:list_class" json:"listClass"`
	// IsDefault 是否默认（Y是 N否）。
	IsDefault string `gorm:"column:is_default" json:"isDefault"`
	Remark    string `gorm:"column:remark" json:"remark"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysDictData) TableName() string {
	return "sys_dict_data"
}
