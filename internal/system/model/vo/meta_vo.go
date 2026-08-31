package vo

import "ruoyi-go-vue-plus/pkg/strutil"

// MetaVo 路由显示信息视图对象，对应 Java MetaVo。
type MetaVo struct {
	Title   string `json:"title"`
	Icon    string `json:"icon"`
	NoCache bool   `json:"noCache"`
	// Link 内链地址，仅当来源是 http(s) 链接时才有值。
	Link string `json:"link"`
	// ActiveMenu 激活菜单路径，仅当以 / 开头时才有值。
	ActiveMenu string `json:"activeMenu"`
}

// NewMetaVo 构造路由元信息。link/activeMenu 不满足格式要求时留空，
// 对应 Java MetaVo 构造器里的 ishttp / startWith("/") 前置判断。
func NewMetaVo(title, icon string, noCache bool, link, activeMenu string) *MetaVo {
	m := &MetaVo{Title: title, Icon: icon, NoCache: noCache}
	if strutil.IsHTTP(link) {
		m.Link = link
	}
	if len(activeMenu) > 0 && activeMenu[0] == '/' {
		m.ActiveMenu = activeMenu
	}
	return m
}
