package vo

// RouterVo 路由配置信息视图对象。
//
// AlwaysShow/Meta 用指针而非值：这两个字段为 null 时整个键不出现，
// 而目录分支置 alwaysShow=true、菜单内链分支置 meta=null，前端靠"键在不在"分流。
type RouterVo struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
	// Hidden 侧边栏是否隐藏；恒输出，bool 的 false 不能被 omitempty 省略。
	Hidden     bool        `json:"hidden"`
	Redirect   string      `json:"redirect,omitempty"`
	Component  string      `json:"component,omitempty"`
	Query      string      `json:"query,omitempty"`
	Ext        string      `json:"ext,omitempty"`
	AlwaysShow *bool       `json:"alwaysShow,omitempty"`
	Meta       *MetaVo     `json:"meta,omitempty"`
	Children   []*RouterVo `json:"children,omitempty"`
}
