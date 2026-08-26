package vo

import (
	"time"
)

// SysMenuVo 菜单权限视图对象，对应 Java SysMenuVo。
type SysMenuVo struct {
	MenuID     int64  `json:"menuId"`
	MenuName   string `json:"menuName"`
	ParentID   int64  `json:"parentId"`
	OrderNum   int    `json:"orderNum"`
	Path       string `json:"path"`
	Component  string `json:"component"`
	QueryParam string `json:"queryParam"`
	// IsFrame 是否为外链（Y是 N否）。
	IsFrame string `json:"isFrame"`
	// IsCache 是否缓存（Y缓存 N不缓存）。
	IsCache string `json:"isCache"`
	// MenuType 菜单类型（M目录 C菜单 F按钮）。
	MenuType string `json:"menuType"`
	// Visible 显示状态（0显示 1隐藏）。
	Visible string `json:"visible"`
	// Status 菜单状态（0正常 1停用）。
	Status     string     `json:"status"`
	Perms      string     `json:"perms"`
	Icon       string     `json:"icon"`
	ActiveMenu string     `json:"activeMenu"`
	Ext        string     `json:"ext"`
	CreateDept int64      `json:"createDept"`
	Remark     string     `json:"remark"`
	CreateTime *time.Time `json:"createTime"`
	// Children 子菜单，由 service 层构建菜单树时回填。
	Children []SysMenuVo `json:"children"`
}
