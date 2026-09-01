package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
)

// TestBuildMenusDirBranch 目录含子节点时挂 alwaysShow + noRedirect 并递归。
func TestBuildMenusDirBranch(t *testing.T) {
	dir := &model.SysMenu{MenuID: 1, ParentID: 0, MenuType: "M", IsFrame: "N",
		Path: "system", MenuName: "系统管理", Icon: "system", Visible: "0", IsCache: "N"}
	dir.Children = []*model.SysMenu{{MenuID: 100, ParentID: 1, MenuType: "C", IsFrame: "N",
		Path: "user", Component: "system/user/index", MenuName: "用户管理", Visible: "0", IsCache: "N"}}

	got := MenuSvcApp.BuildMenus([]*model.SysMenu{dir})
	if len(got) != 1 {
		t.Fatalf("路由数 = %d, 期望 1", len(got))
	}
	r := got[0]
	if r.Name != "System1" || r.Path != "/system" || r.Component != "Layout" {
		t.Errorf("name/path/component = %q/%q/%q", r.Name, r.Path, r.Component)
	}
	if r.AlwaysShow == nil || !*r.AlwaysShow || r.Redirect != "noRedirect" {
		t.Errorf("alwaysShow = %v, redirect = %q", r.AlwaysShow, r.Redirect)
	}
	if len(r.Children) != 1 || r.Children[0].Component != "system/user/index" {
		t.Errorf("子路由未按预期递归: %+v", r.Children)
	}
}

// TestBuildMenusMenuFrameBranch 一级菜单要清空自身 meta，把内容下沉到唯一子路由。
func TestBuildMenusMenuFrameBranch(t *testing.T) {
	menu := &model.SysMenu{MenuID: 5, ParentID: 0, MenuType: "C", IsFrame: "N",
		Path: "index", Component: "system/index", MenuName: "首页", Icon: "dashboard", IsCache: "N"}

	got := MenuSvcApp.BuildMenus([]*model.SysMenu{menu})
	r := got[0]
	if r.Path != "/" {
		t.Errorf("path = %q, 期望 /", r.Path)
	}
	if r.Meta != nil {
		t.Errorf("一级菜单 meta 应为 nil(不序列化), 实际 %+v", r.Meta)
	}
	if len(r.Children) != 1 {
		t.Fatalf("子路由数 = %d, 期望 1", len(r.Children))
	}
	child := r.Children[0]
	if child.Path != "index" || child.Name != "Index5" || child.Component != "system/index" {
		t.Errorf("子路由 = %q/%q/%q", child.Path, child.Name, child.Component)
	}
	if child.Meta == nil || child.Meta.Title != "首页" {
		t.Errorf("子路由 meta 未承载父菜单信息: %+v", child.Meta)
	}
}

// TestBuildMenusInnerLinkBranch 一级内链拆成 "/" + InnerLink 子路由，原地址进 meta.link。
func TestBuildMenusInnerLinkBranch(t *testing.T) {
	link := &model.SysMenu{MenuID: 9, ParentID: 0, MenuType: "M", IsFrame: "N",
		Path: "http://www.ruoyi.vip", MenuName: "若依官网", Icon: "guide"}

	r := MenuSvcApp.BuildMenus([]*model.SysMenu{link})[0]
	if r.Path != "/" {
		t.Errorf("path = %q, 期望 /", r.Path)
	}
	if len(r.Children) != 1 {
		t.Fatalf("子路由数 = %d, 期望 1", len(r.Children))
	}
	child := r.Children[0]
	if child.Path != "ruoyi/vip" || child.Component != "InnerLink" || child.Name != "Ruoyi/vip9" {
		t.Errorf("子路由 = %q/%q/%q", child.Path, child.Component, child.Name)
	}
	if child.Meta == nil || child.Meta.Link != "http://www.ruoyi.vip" {
		t.Errorf("meta.link 未透传原地址: %+v", child.Meta)
	}
}

// TestMenuRouteName routeName 缺省以 path 兜底，但一级非外链菜单的 routeName 恒为空
// （对齐 model.SysMenu.RouteName），故也退化成 path。
func TestMenuRouteName(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		menuType string
		parentID int64
		want     string
	}{
		// 二级菜单：path 首字母大写。
		{"二级菜单", "user", "C", 1761200000000000001, "User"},
		// 一级目录：同样取首字母大写（IsMenuFrame 只认 menuType=C）。
		{"一级目录", "system", "M", 0, "System"},
		// 一级非外链菜单 RouteName 恒为空，兜底成 path 原值。
		{"一级菜单兜底成 path", "index", "C", 0, "index"},
		{"空 path", "", "C", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := menuRouteName(tt.path, tt.menuType, tt.parentID); got != tt.want {
				t.Errorf("menuRouteName(%q, %q, %d) = %q, 期望 %q",
					tt.path, tt.menuType, tt.parentID, got, tt.want)
			}
		})
	}
}

// TestBuildMenuUpdateColumns 可编辑字段一律写（让前端能清空 component/perms 等），
// char(1) 控制字段缺省则跳过（漏传不该把 'Y'/'0' 刷成空串）。
func TestBuildMenuUpdateColumns(t *testing.T) {
	tests := []struct {
		name        string
		in          *bo.SysMenuBo
		wantPresent map[string]any
		wantAbsent  []string
	}{
		{
			name: "全字段",
			in: &bo.SysMenuBo{
				ParentID: 1761200000000000001, MenuName: "用户管理", OrderNum: 1,
				Path: "user", Component: "system/user/index", QueryParam: "a=1",
				IsFrame: "N", IsCache: "Y", MenuType: "C", Visible: "0", Status: "0",
				Perms: "system:user:list", Icon: "user", ActiveMenu: "/system/user",
				Ext: "{}", Remark: "备注",
			},
			wantPresent: map[string]any{
				"parent_id": int64(1761200000000000001), "menu_name": "用户管理",
				"order_num": 1, "path": "user", "component": "system/user/index",
				"query_param": "a=1", "is_frame": "N", "is_cache": "Y", "menu_type": "C",
				"visible": "0", "status": "0", "perms": "system:user:list",
				"icon": "user", "active_menu": "/system/user", "ext": "{}", "remark": "备注",
			},
		},
		{
			// 目录改按钮时必须能清掉组件路径与权限标识，空串要写进库。
			name: "清空组件与权限",
			in: &bo.SysMenuBo{
				MenuName: "按钮", MenuType: "F", Component: "", Perms: "",
				QueryParam: "", ActiveMenu: "", Remark: "",
			},
			wantPresent: map[string]any{
				"component": "", "perms": "", "query_param": "",
				"active_menu": "", "remark": "",
			},
		},
		{
			// 六个控制字段缺省视为不改，不能落进更新列。
			name: "控制字段缺省",
			in:   &bo.SysMenuBo{MenuName: "用户管理", MenuType: "C"},
			wantAbsent: []string{
				"icon", "ext", "is_frame", "is_cache", "visible", "status",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMenuUpdateColumns(tt.in)

			for k, want := range tt.wantPresent {
				v, ok := got[k]
				if !ok {
					t.Errorf("更新列缺少 %q", k)
					continue
				}
				if !reflect.DeepEqual(v, want) {
					t.Errorf("更新列 %q = %#v, 期望 %#v", k, v, want)
				}
			}
			for _, k := range tt.wantAbsent {
				if _, ok := got[k]; ok {
					t.Errorf("更新列不应含 %q", k)
				}
			}
			// 审计字段由 pkg/repository 的回调注入。
			for _, k := range []string{"update_by", "update_time", "create_by", "create_time"} {
				if _, ok := got[k]; ok {
					t.Errorf("更新列不应含审计字段 %q", k)
				}
			}
		})
	}
}

// TestBuildMenuTreeSelect 下拉树须与 hutool 的扁平 JSON 契约一致：
// id/parentId/label/weight 加四个 Extra 键，叶子不出 children。
func TestBuildMenuTreeSelect(t *testing.T) {
	menus := []*vo.SysMenuVo{
		{MenuID: 1, ParentID: 0, MenuName: "系统管理", OrderNum: 1,
			MenuType: "M", Icon: "system", Visible: "0", Status: "0"},
		{MenuID: 100, ParentID: 1, MenuName: "用户管理", OrderNum: 1,
			MenuType: "C", Icon: "user", Visible: "0", Status: "0"},
	}

	got, err := json.Marshal(MenuSvcApp.BuildMenuTreeSelect(menus))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":1,"parentId":0,"label":"系统管理","weight":1,` +
		`"icon":"system","menuType":"M","status":"0","visible":"0",` +
		`"children":[{"id":100,"parentId":1,"label":"用户管理","weight":1,` +
		`"icon":"user","menuType":"C","status":"0","visible":"0"}]}]`
	if string(got) != want {
		t.Errorf("下拉树 JSON 不符\n got: %s\nwant: %s", got, want)
	}
}

// TestRouteConfigUniqueSameLevelPathConflict 同级下路由路径必须唯一，
// 否则前端同一父路由下会注册两条同 path 子路由。
func TestRouteConfigUniqueSameLevelPathConflict(t *testing.T) {
	candidates := []*model.SysMenu{
		{MenuID: 100, ParentID: 1, Path: "user", MenuType: "C", MenuName: "用户管理"},
	}
	b := &bo.SysMenuBo{ParentID: 1, Path: "user", MenuType: "C", MenuName: "新用户管理"}

	if routeConfigUnique(b, menuRouteName(b.Path, b.MenuType, b.ParentID), candidates) {
		t.Error("同级同 path 应判为冲突")
	}
}

// TestRouteConfigUniqueDifferentLevelPathAllowed 跨级同 path 不冲突：
// /system/user 与 /monitor/user 在前端是两条不同路由。
func TestRouteConfigUniqueDifferentLevelPathAllowed(t *testing.T) {
	// 库中是二级菜单 path=user（parent=1），新增的挂在 parent=2 下。
	// 两者 routeName 都是 "User" 且 menuType 同为 C——这正是 Java 第三条规则要拦的，
	// 故此处用不同 menuType 隔开，单验"路径跨级不冲突"。
	candidates := []*model.SysMenu{
		{MenuID: 100, ParentID: 1, Path: "user", MenuType: "M", MenuName: "用户目录"},
	}
	b := &bo.SysMenuBo{ParentID: 2, Path: "user", MenuType: "C", MenuName: "用户管理"}

	if !routeConfigUnique(b, menuRouteName(b.Path, b.MenuType, b.ParentID), candidates) {
		t.Error("跨级同 path 且类型不同，应放行")
	}
}

// TestRouteConfigUniqueRootPathConflict 根目录下的路径是一级路由，必须全局唯一。
//
// 注意它实际命中的是第一条规则（同级 path 冲突）——parentID 同为 0 时两条规则重合，
// Java 侧的根目录分支同样不可达。这里仍单列一例，钉住"根目录同 path 被拒"这个结果，
// 与它走哪条分支无关。
func TestRouteConfigUniqueRootPathConflict(t *testing.T) {
	candidates := []*model.SysMenu{
		{MenuID: 1, ParentID: 0, Path: "system", MenuType: "M", MenuName: "系统管理"},
	}
	b := &bo.SysMenuBo{ParentID: 0, Path: "system", MenuType: "M", MenuName: "系统管理2"}

	if routeConfigUnique(b, menuRouteName(b.Path, b.MenuType, b.ParentID), candidates) {
		t.Error("根目录同 path 应判为冲突")
	}
}

// TestRouteConfigUniqueRouteNameConflict 路由 name 是 vue-router 的全局键，
// 同类型下重名会让后注册的覆盖前者——即便父级不同也要拦。
func TestRouteConfigUniqueRouteNameConflict(t *testing.T) {
	candidates := []*model.SysMenu{
		{MenuID: 100, ParentID: 1, Path: "user", MenuType: "C", MenuName: "用户管理"},
	}
	// 挂在不同父级下，但 routeName 同为 "User" 且类型同为 C。
	b := &bo.SysMenuBo{ParentID: 2, Path: "user", MenuType: "C", MenuName: "另一个用户管理"}

	if routeConfigUnique(b, menuRouteName(b.Path, b.MenuType, b.ParentID), candidates) {
		t.Error("同类型下 routeName 重名应判为冲突")
	}
}

// TestRouteConfigUniqueIsCaseInsensitive 路径比对大小写不敏感（对齐 Java
// equalsAnyIgnoreCase）：vue-router 的 path 在多数部署下不区分大小写，
// 放过 User/user 会造出两条实际同址的路由。
func TestRouteConfigUniqueIsCaseInsensitive(t *testing.T) {
	candidates := []*model.SysMenu{
		{MenuID: 100, ParentID: 1, Path: "user", MenuType: "C", MenuName: "用户管理"},
	}
	b := &bo.SysMenuBo{ParentID: 1, Path: "USER", MenuType: "C", MenuName: "大写用户"}

	if routeConfigUnique(b, menuRouteName(b.Path, b.MenuType, b.ParentID), candidates) {
		t.Error("path 比对应大小写不敏感")
	}
}

// TestRouteConfigUniqueExcludesSelf 修改时命中自己不算冲突，
// 否则任何一次"只改菜单名"的保存都会被自己挡下。
func TestRouteConfigUniqueExcludesSelf(t *testing.T) {
	candidates := []*model.SysMenu{
		{MenuID: 100, ParentID: 1, Path: "user", MenuType: "C", MenuName: "用户管理"},
	}
	b := &bo.SysMenuBo{MenuID: 100, ParentID: 1, Path: "user", MenuType: "C",
		MenuName: "用户管理(改名)"}

	if !routeConfigUnique(b, menuRouteName(b.Path, b.MenuType, b.ParentID), candidates) {
		t.Error("命中自身不应算冲突")
	}
}

// TestRouteConfigUniqueEmptyCandidates 候选集为空即无冲突。
func TestRouteConfigUniqueEmptyCandidates(t *testing.T) {
	b := &bo.SysMenuBo{ParentID: 0, Path: "system", MenuType: "M", MenuName: "系统管理"}
	if !routeConfigUnique(b, "System", nil) {
		t.Error("候选集为空应放行")
	}
}
