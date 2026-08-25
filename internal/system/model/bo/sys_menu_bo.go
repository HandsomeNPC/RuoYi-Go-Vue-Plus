package bo

import (
	systemmodel "ruoyi-go-vue-plus/internal/system/model"
)

// SysMenuBo 菜单权限业务对象（入参），对应 Java SysMenuBo。
type SysMenuBo struct {
	MenuID     int64  `json:"menuId"`
	ParentID   int64  `json:"parentId"`
	MenuName   string `json:"menuName" binding:"required,max=50"`
	OrderNum   int    `json:"orderNum" binding:"required"`
	Path       string `json:"path" binding:"omitempty,max=200"`
	Component  string `json:"component" binding:"omitempty,max=200"`
	QueryParam string `json:"queryParam"`
	// IsFrame 是否为外链（Y是 N否）。
	IsFrame string `json:"isFrame"`
	// IsCache 是否缓存（Y缓存 N不缓存）。
	IsCache string `json:"isCache"`
	// MenuType 菜单类型（M目录 C菜单 F按钮）。
	MenuType string `json:"menuType" binding:"required"`
	// Visible 显示状态（0显示 1隐藏）。
	Visible string `json:"visible"`
	// Status 菜单状态（0正常 1停用）。
	Status     string `json:"status"`
	Perms      string `json:"perms" binding:"omitempty,max=100"`
	Icon       string `json:"icon"`
	ActiveMenu string `json:"activeMenu" binding:"omitempty,max=255"`
	Ext        string `json:"ext" binding:"omitempty,max=2000"`
	Remark     string `json:"remark"`
}

// ToSysMenu 把 BO 转成实体。
func (b *SysMenuBo) ToSysMenu() *systemmodel.SysMenu {
	if b == nil {
		return nil
	}
	return &systemmodel.SysMenu{
		MenuID:     b.MenuID,
		ParentID:   b.ParentID,
		MenuName:   b.MenuName,
		OrderNum:   b.OrderNum,
		Path:       b.Path,
		Component:  b.Component,
		QueryParam: b.QueryParam,
		IsFrame:    b.IsFrame,
		IsCache:    b.IsCache,
		MenuType:   b.MenuType,
		Visible:    b.Visible,
		Status:     b.Status,
		Perms:      b.Perms,
		Icon:       b.Icon,
		ActiveMenu: b.ActiveMenu,
		Ext:        b.Ext,
		Remark:     b.Remark,
	}
}
