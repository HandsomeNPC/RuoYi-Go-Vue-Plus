package vo

import (
	"time"
)

// SysConfigVo 参数配置视图对象，对应 Java SysConfigVo。
type SysConfigVo struct {
	ConfigID    int64  `json:"configId" excel:"参数主键"`
	ConfigName  string `json:"configName" excel:"参数名称"`
	ConfigKey   string `json:"configKey" excel:"参数键名"`
	ConfigValue string `json:"configValue" excel:"参数键值"`
	// ConfigType 系统内置（Y是 N否）。
	// 导出时按 excelDict 转成标签，对齐 Java @ExcelDictFormat(dictType = "sys_yes_no")。
	ConfigType string     `json:"configType" excel:"系统内置" excelDict:"Y=是,N=否"`
	Remark     string     `json:"remark" excel:"备注"`
	CreateTime *time.Time `json:"createTime" excel:"创建时间"`
}
