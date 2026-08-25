package vo

// RouterVo 路由配置信息视图对象，对应 Java RouterVo。
type RouterVo struct {
	Name       string `json:"name,omitempty"`
	Path       string `json:"path,omitempty"`
	Hidden     bool   `json:"hidden,omitempty"`
	Redirect   string `json:"redirect,omitempty"`
	Component  string `json:"component,omitempty"`
	Query      string `json:"query,omitempty"`
	Ext        string `json:"ext,omitempty"`
	AlwaysShow bool   `json:"alwaysShow,omitempty"`
	Meta       MetaVo `json:"meta,omitempty"`
	// Children 子路由，由 service 构建路由树时回填。
	Children []RouterVo `json:"children,omitempty"`
}
