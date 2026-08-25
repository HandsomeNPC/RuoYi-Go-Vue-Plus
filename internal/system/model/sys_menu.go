package model

// SysMenu 菜单权限表（sys_menu），对应 Java org.dromara.system.domain.SysMenu。
//
// 仅迁移实体字段；Java SysMenu 上的路由推导方法（getRouteName/getRouterPath/
// getComponentInfo 等）依赖前端路由构建逻辑，后续迁移到 service/vo 层实现。
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
