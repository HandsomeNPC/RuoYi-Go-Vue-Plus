package model

import "testing"

// TestRouterPath 覆盖 Java getRouterPath 的四条分支。
func TestRouterPath(t *testing.T) {
	tests := []struct {
		name string
		menu SysMenu
		want string
	}{
		{"一级目录补前导斜杠", SysMenu{ParentID: 0, MenuType: "M", IsFrame: "N", Path: "system"}, "/system"},
		{"一级菜单挂根路由", SysMenu{ParentID: 0, MenuType: "C", IsFrame: "N", Path: "index"}, "/"},
		{"二级菜单原样", SysMenu{ParentID: 1, MenuType: "C", IsFrame: "N", Path: "user"}, "user"},
		{"二级内链转路径", SysMenu{ParentID: 1, MenuType: "C", IsFrame: "N", Path: "http://www.ruoyi.vip/abc"},
			"ruoyi/vip/abc"},
		{"一级外链目录不补斜杠", SysMenu{ParentID: 0, MenuType: "M", IsFrame: "Y", Path: "http://x.com"},
			"http://x.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.menu.RouterPath(); got != tt.want {
				t.Errorf("RouterPath() = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

// TestComponentInfo 覆盖 Java getComponentInfo 的四条分支。
func TestComponentInfo(t *testing.T) {
	tests := []struct {
		name string
		menu SysMenu
		want string
	}{
		{"一级目录用 Layout", SysMenu{ParentID: 0, MenuType: "M", IsFrame: "N"}, "Layout"},
		{"一级菜单忽略自身 component", SysMenu{ParentID: 0, MenuType: "C", IsFrame: "N",
			Component: "system/index"}, "Layout"},
		{"二级菜单用自身 component", SysMenu{ParentID: 1, MenuType: "C", IsFrame: "N",
			Component: "system/user/index"}, "system/user/index"},
		{"二级空 component 内链", SysMenu{ParentID: 1, MenuType: "C", IsFrame: "N",
			Path: "https://x.com"}, "InnerLink"},
		{"二级目录用 ParentView", SysMenu{ParentID: 1, MenuType: "M", IsFrame: "N"}, "ParentView"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.menu.ComponentInfo(); got != tt.want {
				t.Errorf("ComponentInfo() = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

// TestRouteName 一级非外链菜单的 name 由 path 兜底，故本方法返回空。
func TestRouteName(t *testing.T) {
	menuFrame := SysMenu{ParentID: 0, MenuType: "C", IsFrame: "N", Path: "index"}
	if got := menuFrame.RouteName(); got != "" {
		t.Errorf("一级菜单 RouteName() = %q, 期望空串", got)
	}
	dir := SysMenu{ParentID: 0, MenuType: "M", IsFrame: "N", Path: "system"}
	if got, want := dir.RouteName(), "System"; got != want {
		t.Errorf("RouteName() = %q, 期望 %q", got, want)
	}
}

// TestInnerLinkReplaceEach 替换必须单趟完成：先替换出的 / 不能再被后续规则命中。
func TestInnerLinkReplaceEach(t *testing.T) {
	tests := []struct{ in, want string }{
		{"http://www.ruoyi.vip", "ruoyi/vip"},
		{"https://gitee.com/dromara", "gitee/com/dromara"},
		{"http://localhost:8080/doc", "localhost/8080/doc"},
	}
	for _, tt := range tests {
		if got := InnerLinkReplaceEach(tt.in); got != tt.want {
			t.Errorf("InnerLinkReplaceEach(%q) = %q, 期望 %q", tt.in, got, tt.want)
		}
	}
}
