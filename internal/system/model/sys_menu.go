package model

import (
	"strings"

	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/strutil"
)

// SysMenu 菜单权限表（sys_menu），对应 Java org.dromara.system.domain.SysMenu。
type SysMenu struct {
	MenuID     int64  `gorm:"column:menu_id;primaryKey" json:"menuId"`
	ParentID   int64  `gorm:"column:parent_id" json:"parentId"`
	MenuName   string `gorm:"column:menu_name" json:"menuName"`
	OrderNum   int    `gorm:"column:order_num" json:"orderNum"`
	Path       string `gorm:"column:path" json:"path"`
	Component  string `gorm:"column:component" json:"component"`
	QueryParam string `gorm:"column:query_param" json:"queryParam"`
	// IsFrame 是否为外链（Y是 N否）。
	IsFrame string `gorm:"column:is_frame" json:"isFrame"`
	// IsCache 是否缓存（Y缓存 N不缓存）。
	IsCache string `gorm:"column:is_cache" json:"isCache"`
	// MenuType 类型（M目录 C菜单 F按钮）。
	MenuType string `gorm:"column:menu_type" json:"menuType"`
	// Visible 显示状态（0显示 1隐藏）。
	Visible string `gorm:"column:visible" json:"visible"`
	// Status 菜单状态（0正常 1停用）。
	Status     string `gorm:"column:status" json:"status"`
	Perms      string `gorm:"column:perms" json:"perms"`
	Icon       string `gorm:"column:icon" json:"icon"`
	ActiveMenu string `gorm:"column:active_menu" json:"activeMenu"`
	Ext        string `gorm:"column:ext" json:"ext"`
	Remark     string `gorm:"column:remark" json:"remark"`

	// ParentName 父菜单名称，非表字段，关联查询时回填。
	ParentName string `gorm:"-" json:"parentName,omitempty"`
	// Children 子菜单，非表字段，构建菜单树时由 service 层填充。
	Children []*SysMenu `gorm:"-" json:"children,omitempty"`

	BaseEntity
}

// TableName 显式指定表名。
func (SysMenu) TableName() string {
	return "sys_menu"
}

// innerLinkReplacer 内链地址转路由路径：剥协议头与 www.，再把 . : 换成 /。
// 用 Replacer 而非多次 ReplaceAll，是为对齐 Java replaceEach 的单趟、不重叠替换语义
// （多次 ReplaceAll 会让前一轮产出的字符再被后一轮匹配）。
var innerLinkReplacer = strings.NewReplacer(
	constant.ConstantHTTP, "",
	constant.ConstantHTTPS, "",
	constant.ConstantWWW, "",
	".", "/",
	":", "/",
)

// InnerLinkReplaceEach 内链域名特殊字符替换，对应 Java SysMenu.innerLinkReplaceEach。
func InnerLinkReplaceEach(path string) string {
	return innerLinkReplacer.Replace(path)
}

// RouteName 路由名称，对应 Java SysMenu.getRouteName。一级非外链菜单交由 path 直接兜底，故返回空。
func (m *SysMenu) RouteName() string {
	if m.IsMenuFrame() {
		return ""
	}
	return strutil.Capitalize(m.Path)
}

// RouterPath 路由地址，对应 Java SysMenu.getRouterPath。
func (m *SysMenu) RouterPath() string {
	routerPath := m.Path
	if m.ParentID != constant.ConstantTopParentID && m.IsInnerLink() {
		routerPath = InnerLinkReplaceEach(routerPath)
	}
	switch {
	case m.ParentID == constant.ConstantTopParentID &&
		m.MenuType == constant.MenuTypeDir && m.IsFrame == constant.No:
		routerPath = "/" + m.Path
	case m.IsMenuFrame():
		routerPath = "/"
	}
	return routerPath
}

// ComponentInfo 组件路径，对应 Java SysMenu.getComponentInfo。
func (m *SysMenu) ComponentInfo() string {
	switch {
	case m.Component != "" && !m.IsMenuFrame():
		return m.Component
	case m.Component == "" && m.ParentID != constant.ConstantTopParentID && m.IsInnerLink():
		return constant.ComponentInnerLink
	case m.Component == "" && m.IsParentView():
		return constant.ComponentParentView
	default:
		return constant.ComponentLayout
	}
}

// IsMenuFrame 是否为一级非外链菜单（此类菜单在前端挂到根路由下）。
func (m *SysMenu) IsMenuFrame() bool {
	return m.ParentID == constant.ConstantTopParentID &&
		m.MenuType == constant.MenuTypeMenu && m.IsFrame == constant.No
}

// IsInnerLink 是否为内链组件（非外链但 path 填的是 http 地址）。
func (m *SysMenu) IsInnerLink() bool {
	return m.IsFrame == constant.No && strutil.IsHTTP(m.Path)
}

// IsParentView 是否为 ParentView 组件（非一级目录）。
func (m *SysMenu) IsParentView() bool {
	return m.ParentID != constant.ConstantTopParentID && m.MenuType == constant.MenuTypeDir
}
