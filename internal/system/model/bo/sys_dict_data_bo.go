package bo

// SysDictDataBo 字典数据业务对象（入参），对应 Java SysDictDataBo。
type SysDictDataBo struct {
	DictCode  int64  `json:"dictCode"`
	DictSort  int    `json:"dictSort"`
	DictLabel string `json:"dictLabel" binding:"required,max=100"`
	DictValue string `json:"dictValue" binding:"required,max=100"`
	DictType  string `json:"dictType" binding:"required,max=100"`
	CssClass  string `json:"cssClass" binding:"omitempty,max=100"`
	ListClass string `json:"listClass"`
	// IsDefault 是否默认（Y是 N否）。
	IsDefault  string `json:"isDefault"`
	CreateDept int64  `json:"createDept"`
	Remark     string `json:"remark"`
}
