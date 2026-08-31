package service

import (
	"testing"

	"ruoyi-go-vue-plus/internal/system/model"
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
