// Package constant resource 模块专属常量。
package constant

// sys_oss_config.status 的是否默认标记。
//
// 与 system 的同名常量是各自一份：两边指的是不同表的列，
// 让 resource 不必为两个字面量依赖 system。
const (
	Yes = "Y"
	No  = "N"
)
