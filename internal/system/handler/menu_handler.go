package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// MenuApi 菜单信息接口。
type MenuApi struct{}

var MenuApiApp = new(MenuApi)

// GetRouters 获取当前用户可访问的路由信息。
func (a *MenuApi) GetRouters(c *gin.Context) {
	menus, err := systemservice.MenuSvcApp.SelectMenuTreeByUserId(
		c.Request.Context(), loginhelper.GetUserID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(systemservice.MenuSvcApp.BuildMenus(menus)))
}

// List 查询菜单列表。
//
// 不分页：菜单总量有限，前端拿扁平列表后自行构树整树渲染。
func (a *MenuApi) List(c *gin.Context) {
	var q bo.SysMenuQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	res, err := systemservice.MenuSvcApp.QueryList(c.Request.Context(), q,
		loginhelper.GetUserID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// GetInfo 按主键查菜单详情。
func (a *MenuApi) GetInfo(c *gin.Context) {
	menuID, ok := parseMenuID(c)
	if !ok {
		return
	}

	res, err := systemservice.MenuSvcApp.QueryByID(c.Request.Context(), menuID)
	if err != nil {
		if errors.Is(err, systemservice.ErrMenuNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "菜单不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// TreeSelect 获取菜单下拉树列表。
func (a *MenuApi) TreeSelect(c *gin.Context) {
	var q bo.SysMenuQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	menus, err := systemservice.MenuSvcApp.QueryList(c.Request.Context(), q,
		loginhelper.GetUserID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(systemservice.MenuSvcApp.BuildMenuTreeSelect(menus)))
}

// RoleMenuTreeSelect 加载对应角色的菜单列表树及其选中节点。
//
// 菜单树取的是**当前登录用户**可见的全量菜单（不带筛选条件），选中节点取的是
// 目标角色已授权的菜单——两者角色不同是有意的：授权界面要在自己可见的范围内勾选。
func (a *MenuApi) RoleMenuTreeSelect(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Param("roleId"), 10, 64)
	if err != nil || roleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", c.Param("roleId")))
		return
	}

	ctx := c.Request.Context()
	menus, err := systemservice.MenuSvcApp.QueryList(ctx, bo.SysMenuQueryBo{},
		loginhelper.GetUserID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	checkedKeys, err := systemservice.MenuSvcApp.SelectMenuIDsByRoleID(ctx, roleID)
	if err != nil {
		if errors.Is(err, systemservice.ErrRoleNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "角色不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	// checkedKeys 兜成空切片：前端读 .length，null 会抛。
	if checkedKeys == nil {
		checkedKeys = []int64{}
	}

	c.JSON(http.StatusOK, response.Ok(vo.MenuTreeSelectVo{
		CheckedKeys: checkedKeys,
		Menus:       systemservice.MenuSvcApp.BuildMenuTreeSelect(menus),
	}))
}

// Add 新增菜单。
func (a *MenuApi) Add(c *gin.Context) {
	var b bo.SysMenuBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.MenuSvcApp.InsertMenu(c.Request.Context(), &b); err != nil {
		if translated := translateMenuValidationError(err, "新增", b.MenuName); translated != nil {
			_ = c.Error(translated)
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改菜单。
func (a *MenuApi) Edit(c *gin.Context) {
	var b bo.SysMenuBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysMenuBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.MenuID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "菜单主键不能为空", ""))
		return
	}

	if err := systemservice.MenuSvcApp.UpdateMenu(c.Request.Context(), &b); err != nil {
		if translated := translateMenuValidationError(err, "修改", b.MenuName); translated != nil {
			_ = c.Error(translated)
			return
		}
		if errors.Is(err, systemservice.ErrMenuNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "菜单不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 删除单个菜单。
func (a *MenuApi) Remove(c *gin.Context) {
	menuID, ok := parseMenuID(c)
	if !ok {
		return
	}

	// 「存在子菜单」「菜单已分配」两类拦截由 service 以 ServiceError 抛出，此处无需另行翻译。
	if err := systemservice.MenuSvcApp.DeleteMenuByID(c.Request.Context(), menuID); err != nil {
		if errors.Is(err, systemservice.ErrMenuNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "菜单不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// CascadeRemove 批量级联删除菜单，同时清理角色授权。
// 主键串以逗号分隔。
func (a *MenuApi) CascadeRemove(c *gin.Context) {
	ids, err := parseIDs(c.Param("menuIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "菜单主键不能为空", c.Param("menuIds")))
		return
	}

	if err := systemservice.MenuSvcApp.DeleteMenuByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// translateMenuValidationError 把新增/修改共用的三类校验哨兵错误翻成前端文案，
// 命中返回可直接 c.Error 的错误，未命中返回 nil 交调用方兜底。
//
// action 取"新增"/"修改"，用作两个分支的提示前缀。
func translateMenuValidationError(err error, action, menuName string) error {
	switch {
	case errors.Is(err, systemservice.ErrMenuNameExists):
		return errs.New(response.CodeFail,
			fmt.Sprintf("%s菜单'%s'失败，菜单名称已存在", action, menuName), "")
	case errors.Is(err, systemservice.ErrMenuFrameNeedHTTP):
		return errs.New(response.CodeFail,
			fmt.Sprintf("%s菜单'%s'失败，地址必须以http(s)://开头", action, menuName), "")
	case errors.Is(err, systemservice.ErrMenuParentIsSelf):
		return errs.New(response.CodeFail,
			fmt.Sprintf("%s菜单'%s'失败，上级菜单不能选择自己", action, menuName), "")
	case errors.Is(err, systemservice.ErrMenuRouteConflict):
		return errs.New(response.CodeFail,
			fmt.Sprintf("%s菜单'%s'失败，路由名称或地址已存在", action, menuName), "")
	}
	return nil
}

// parseMenuID 取路径参数里的菜单主键，非法时已登记错误，返回 false 让调用方直接 return。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func parseMenuID(c *gin.Context) (int64, bool) {
	menuID, err := strconv.ParseInt(c.Param("menuId"), 10, 64)
	if err != nil || menuID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "菜单主键不能为空", c.Param("menuId")))
		return 0, false
	}
	return menuID, true
}
