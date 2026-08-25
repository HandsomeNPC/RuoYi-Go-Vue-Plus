package vo

// MetaVo 路由显示信息视图对象，对应 Java MetaVo。
type MetaVo struct {
	Title      string `json:"title"`
	Icon       string `json:"icon"`
	NoCache    bool   `json:"noCache"`
	Link       string `json:"link"`
	ActiveMenu string `json:"activeMenu"`
}
