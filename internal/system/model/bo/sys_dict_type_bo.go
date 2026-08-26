package bo

// SysDictTypeBo 字典类型业务对象（入参），对应 Java SysDictTypeBo。
type SysDictTypeBo struct {
	DictID   int64  `json:"dictId"`
	DictName string `json:"dictName" binding:"required,max=100"`
	DictType string `json:"dictType" binding:"required,max=100"`
	Remark   string `json:"remark"`
}
