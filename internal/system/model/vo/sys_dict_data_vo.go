package vo

import (
	"time"
)

// SysDictDataVo 字典数据视图对象，对应 Java SysDictDataVo。
type SysDictDataVo struct {
	DictCode  int64  `json:"dictCode" excel:"字典编码"`
	DictSort  int    `json:"dictSort" excel:"字典排序"`
	DictLabel string `json:"dictLabel" excel:"字典标签"`
	DictValue string `json:"dictValue" excel:"字典键值"`
	DictType  string `json:"dictType" excel:"字典类型"`
	// CssClass/ListClass 无 excel tag，与 Java 未标 @ExcelProperty 一致，不导出。
	CssClass  string `json:"cssClass"`
	ListClass string `json:"listClass"`
	// IsDefault 是否默认（Y是 N否）。
	// 导出时按 excelDict 转成标签，对齐 Java @ExcelDictFormat(dictType = "sys_yes_no")。
	IsDefault  string     `json:"isDefault" excel:"是否默认" excelDict:"Y=是,N=否"`
	Remark     string     `json:"remark" excel:"备注"`
	CreateTime *time.Time `json:"createTime" excel:"创建时间"`
}
