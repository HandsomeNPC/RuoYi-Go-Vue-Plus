package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/excel"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// RoleApi 角色信息接口。
type RoleApi struct{}

var RoleApiApp = new(RoleApi)

// List 分页查询角色列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *RoleApi) List(c *gin.Context) {
	var q bo.SysRoleQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.RoleSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出角色列表为 xlsx 附件。
// 走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件，
// 故用 ShouldBind 同时吃 form body 与 query。
//
// 响应体是二进制附件，不返回 response.R——见 pkg/excel 的说明。
// 多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *RoleApi) Export(c *gin.Context) {
	var q bo.SysRoleQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := systemservice.RoleSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "角色数据"); err != nil {
		_ = c.Error(err)
		return
	}
}

// GetInfo 按主键查角色详情。
// 先做数据权限校验，校验失败直接拦在 selectRoleById 之前，
// 故角色不存在时落在"没有权限访问部分角色数据"而非 404。
func (a *RoleApi) GetInfo(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Param("roleId"), 10, 64)
	if err != nil || roleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", c.Param("roleId")))
		return
	}

	ctx := c.Request.Context()
	if err := systemservice.RoleSvcApp.CheckRoleDataScope(ctx, loginhelper.GetUserID(c), roleID); err != nil {
		_ = c.Error(err)
		return
	}
	res, err := systemservice.RoleSvcApp.QueryByID(ctx, roleID)
	if err != nil {
		if errors.Is(err, systemservice.ErrRoleNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "角色不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增角色。
func (a *RoleApi) Add(c *gin.Context) {
	var b bo.SysRoleBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.RoleSvcApp.InsertRole(c.Request.Context(), &b); err != nil {
		_ = c.Error(translateRoleError(err, "新增", b.RoleName))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改角色基础信息（不含菜单/数据权限）。
func (a *RoleApi) Edit(c *gin.Context) {
	var b bo.SysRoleBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysRoleBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.RoleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", ""))
		return
	}

	ctx := c.Request.Context()
	if err := systemservice.RoleSvcApp.CheckRoleDataScope(ctx, loginhelper.GetUserID(c), b.RoleID); err != nil {
		_ = c.Error(err)
		return
	}
	rows, err := systemservice.RoleSvcApp.UpdateRoleBaseInfo(ctx, &b)
	if err != nil {
		_ = c.Error(translateRoleError(err, "修改", b.RoleName))
		return
	}
	// rows==0 时报失败：值与库中相同时 MySQL 报 0 行。
	if rows == 0 {
		_ = c.Error(errs.New(response.CodeFail,
			fmt.Sprintf("修改角色'%s'失败，请联系管理员", b.RoleName), ""))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// EditPermission 修改角色权限（菜单 + 数据权限）。
func (a *RoleApi) EditPermission(c *gin.Context) {
	var b bo.SysRoleBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	if b.RoleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", ""))
		return
	}

	ctx := c.Request.Context()
	if err := systemservice.RoleSvcApp.CheckRoleDataScope(ctx, loginhelper.GetUserID(c), b.RoleID); err != nil {
		_ = c.Error(err)
		return
	}
	rows, err := systemservice.RoleSvcApp.UpdateRolePermission(ctx, &b)
	if err != nil {
		_ = c.Error(translateRoleError(err, "修改", b.RoleName))
		return
	}
	if rows == 0 {
		_ = c.Error(errs.New(response.CodeFail,
			fmt.Sprintf("修改角色'%s'权限失败，请联系管理员", b.RoleName), ""))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// ChangeStatus 修改角色状态。
func (a *RoleApi) ChangeStatus(c *gin.Context) {
	var b bo.SysRoleBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	if b.RoleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", ""))
		return
	}

	ctx := c.Request.Context()
	if err := systemservice.RoleSvcApp.CheckRoleAllowed(ctx, &b); err != nil {
		_ = c.Error(err)
		return
	}
	if err := systemservice.RoleSvcApp.CheckRoleDataScope(ctx, loginhelper.GetUserID(c), b.RoleID); err != nil {
		_ = c.Error(err)
		return
	}
	rows, err := systemservice.RoleSvcApp.UpdateRoleStatus(ctx, b.RoleID, b.Status)
	if err != nil {
		// 「角色已分配，不能禁用」由 service 以 ServiceError 抛出，此处无需另行翻译。
		_ = c.Error(err)
		return
	}
	if rows == 0 {
		_ = c.Error(errs.New(response.CodeFail,
			fmt.Sprintf("修改角色'%s'状态失败，请联系管理员", b.RoleName), ""))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除角色。主键串以逗号分隔。
func (a *RoleApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("roleIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", c.Param("roleIds")))
		return
	}

	// 「已分配,不能删除」「没有权限访问部分角色数据」「不允许操作超级管理员角色」
	// 三类拦截由 service 以 ServiceError 抛出，此处无需另行翻译。
	if err := systemservice.RoleSvcApp.DeleteRoleByIDs(
		c.Request.Context(), loginhelper.GetUserID(c), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// OptionSelect 获取角色选择框列表。
// roleIds 可缺省：传则按主键过滤且只取启用角色，不传返回全部启用角色。
// 数组既可能以 roleIds=1&roleIds=2 下发，也可能是 roleIds=1,2，两种都要吃下。
func (a *RoleApi) OptionSelect(c *gin.Context) {
	var roleIDs []int64
	if raw := strings.Join(c.QueryArray("roleIds"), ","); strings.Trim(raw, ",") != "" {
		parsed, err := parseIDs(raw)
		if err != nil {
			_ = c.Error(errs.New(response.CodeBadRequest, "角色主键有误", raw))
			return
		}
		roleIDs = parsed
	}

	res, err := systemservice.RoleSvcApp.SelectByIDs(c.Request.Context(), roleIDs)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// AllocatedList 查询已分配该角色的用户列表。
func (a *RoleApi) AllocatedList(c *gin.Context) {
	var q bo.SysUserQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.UserSvcApp.SelectAllocatedList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// UnallocatedList 查询未分配该角色的用户列表。
func (a *RoleApi) UnallocatedList(c *gin.Context) {
	var q bo.SysUserQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.UserSvcApp.SelectUnallocatedList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// CancelAuthUser 取消单个用户的角色授权。
func (a *RoleApi) CancelAuthUser(c *gin.Context) {
	var ur model.SysUserRole
	if err := c.ShouldBindJSON(&ur); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := systemservice.RoleSvcApp.DeleteAuthUser(
		c.Request.Context(), loginhelper.GetUserID(c), ur.RoleID, ur.UserID)
	if err != nil {
		// 「不允许修改当前用户角色」由 service 以 ServiceError 抛出，此处无需另行翻译。
		_ = c.Error(err)
		return
	}
	if rows == 0 {
		_ = c.Error(errs.New(response.CodeFail, "取消授权失败，该用户未分配此角色", ""))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// CancelAuthUserAll 批量取消角色下的用户授权。
// roleId 与 userIds 走 query 参数。
func (a *RoleApi) CancelAuthUserAll(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Query("roleId"), 10, 64)
	if err != nil || roleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", c.Query("roleId")))
		return
	}
	userIDs, ok := parseUserIDsQuery(c)
	if !ok {
		return
	}

	rows, err := systemservice.RoleSvcApp.DeleteAuthUsers(
		c.Request.Context(), loginhelper.GetUserID(c), roleID, userIDs)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if rows == 0 {
		_ = c.Error(errs.New(response.CodeFail, "操作失败", ""))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// SelectAuthUserAll 批量给角色追加用户授权。
// 先做数据权限校验，再授权并踢受影响用户下线。
func (a *RoleApi) SelectAuthUserAll(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Query("roleId"), 10, 64)
	if err != nil || roleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", c.Query("roleId")))
		return
	}
	userIDs, ok := parseUserIDsQuery(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := systemservice.RoleSvcApp.CheckRoleDataScope(ctx, loginhelper.GetUserID(c), roleID); err != nil {
		_ = c.Error(err)
		return
	}
	rows, err := systemservice.RoleSvcApp.InsertAuthUsers(ctx, roleID, userIDs)
	if err != nil {
		// 「不允许修改当前用户角色」由 service 以 ServiceError 抛出，此处无需另行翻译。
		_ = c.Error(err)
		return
	}
	if rows == 0 {
		_ = c.Error(errs.New(response.CodeFail, "操作失败", ""))
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// DeptTreeSelect 获取对应角色的部门树及选中节点。
// 选中节点取该角色的部门勾选；树取全量部门下拉树。
func (a *RoleApi) DeptTreeSelect(c *gin.Context) {
	roleID, err := strconv.ParseInt(c.Param("roleId"), 10, 64)
	if err != nil || roleID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "角色主键不能为空", c.Param("roleId")))
		return
	}

	ctx := c.Request.Context()
	checkedKeys, err := systemservice.RoleSvcApp.SelectDeptIDsByRoleID(ctx, roleID)
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
	depts, err := systemservice.DeptSvcApp.SelectDeptTreeList(ctx, bo.SysDeptQueryBo{})
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(vo.DeptTreeSelectVo{
		CheckedKeys: checkedKeys,
		Depts:       depts,
	}))
}

// translateRoleError 把新增/修改共用的几类校验哨兵错误翻成前端文案，
// 命中返回可直接 c.Error 的错误，未命中返回 nil 交调用方兜底。
//
// action 取"新增"/"修改"，用作两个分支的提示前缀。
func translateRoleError(err error, action, roleName string) error {
	switch {
	case errors.Is(err, systemservice.ErrRoleNameExists):
		return errs.New(response.CodeFail,
			fmt.Sprintf("%s角色'%s'失败，角色名称已存在", action, roleName), "")
	case errors.Is(err, systemservice.ErrRoleKeyExists):
		return errs.New(response.CodeFail,
			fmt.Sprintf("%s角色'%s'失败，角色权限已存在", action, roleName), "")
	case errors.Is(err, systemservice.ErrRoleNotFound):
		return errs.New(response.CodeNotFound, "角色不存在", "")
	}
	return err
}

// parseUserIDsQuery 解析 query 上的 userIds 数组参数（形如 userIds=1&userIds=2 或 userIds=1,2）。
// 空集合合法（selectAll 空批视作成功），返回 (nil, true)。
// 任一段非法即整体拒绝并登记错误，返回 ok=false 让调用方直接 return。
func parseUserIDsQuery(c *gin.Context) ([]int64, bool) {
	raw := strings.Join(c.QueryArray("userIds"), ",")
	if strings.Trim(raw, ",") == "" {
		return nil, true
	}
	userIDs, err := parseIDs(raw)
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "用户主键有误", raw))
		return nil, false
	}
	return userIDs, true
}
