package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// DeptApi 部门信息接口（对应 Java SysDeptController）。
type DeptApi struct{}

// DeptApiApp 包级实例。
var DeptApiApp = new(DeptApi)

// List 查询部门列表。
//
// 不分页：部门总量有限，前端拿扁平列表后自行 listToTree 整树渲染，与 Java 一致。
func (a *DeptApi) List(c *gin.Context) {
	var q bo.SysDeptQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	res, err := systemservice.DeptSvcApp.QueryList(c.Request.Context(), q)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// ExcludeChild 查询部门列表并排除指定节点及其后代（对应 Java excludeChild）。
// 用于编辑部门时的上级部门下拉框——把自己和自己的子树排掉，避免选出环。
func (a *DeptApi) ExcludeChild(c *gin.Context) {
	deptID, ok := parseDeptID(c)
	if !ok {
		return
	}

	res, err := systemservice.DeptSvcApp.QueryListExcludeChild(c.Request.Context(), deptID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// GetInfo 按主键查部门详情。
func (a *DeptApi) GetInfo(c *gin.Context) {
	deptID, ok := parseDeptID(c)
	if !ok {
		return
	}

	ctx := c.Request.Context()
	if err := systemservice.DeptSvcApp.CheckDeptDataScope(ctx,
		loginhelper.GetUserID(c), deptID); err != nil {
		_ = c.Error(err)
		return
	}

	res, err := systemservice.DeptSvcApp.SelectByID(ctx, deptID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// 查不到时回 data: null 而非 404，对齐 Java R.ok(selectDeptById(...)) 的形态。
	c.JSON(http.StatusOK, response.Ok(res))
}

// OptionSelect 获取部门选择框列表（对应 Java optionselect）。
// deptIds 可缺省，缺省即返回全部启用部门。
func (a *DeptApi) OptionSelect(c *gin.Context) {
	var ids []int64
	// 与 Java 的 @RequestParam(required = false) Long[] 同形：数组既可能以
	// deptIds=1&deptIds=2 下发，也可能是 deptIds=1,2，两种都要吃下。
	// 空值（?deptIds=）按缺省处理，不当成非法入参——Java 侧同样得到空数组而非报错。
	if raw := strings.Join(c.QueryArray("deptIds"), ","); strings.Trim(raw, ",") != "" {
		parsed, err := parseIDs(raw)
		if err != nil {
			_ = c.Error(errs.New(response.CodeBadRequest, "部门主键有误", raw))
			return
		}
		ids = parsed
	}

	res, err := systemservice.DeptSvcApp.QueryByIDs(c.Request.Context(), ids)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增部门。
func (a *DeptApi) Add(c *gin.Context) {
	var b bo.SysDeptBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.DeptSvcApp.InsertDept(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrDeptNameExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("新增部门'%s'失败，部门名称已存在", b.DeptName), ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改部门。
func (a *DeptApi) Edit(c *gin.Context) {
	var b bo.SysDeptBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysDeptBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.DeptID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "部门主键不能为空", ""))
		return
	}

	ctx := c.Request.Context()
	userID := loginhelper.GetUserID(c)
	if err := systemservice.DeptSvcApp.CheckDeptDataScope(ctx, userID, b.DeptID); err != nil {
		_ = c.Error(err)
		return
	}

	if err := systemservice.DeptSvcApp.UpdateDept(ctx, userID, &b); err != nil {
		if errors.Is(err, systemservice.ErrDeptNameExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改部门'%s'失败，部门名称已存在", b.DeptName), ""))
			return
		}
		if errors.Is(err, systemservice.ErrDeptParentIsSelf) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改部门'%s'失败，上级部门不能是自己", b.DeptName), ""))
			return
		}
		if errors.Is(err, systemservice.ErrDeptNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "部门不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 删除部门。与 Java 一致只支持单删——部门有父子关系，批量删无法保证删除顺序。
func (a *DeptApi) Remove(c *gin.Context) {
	deptID, ok := parseDeptID(c)
	if !ok {
		return
	}

	// 默认部门/存在下级/存在用户/存在岗位四类拦截由 service 以 ServiceError 抛出，
	// 此处无需另行翻译。
	if err := systemservice.DeptSvcApp.DeleteDeptByID(c.Request.Context(),
		loginhelper.GetUserID(c), deptID); err != nil {
		if errors.Is(err, systemservice.ErrDeptNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "部门不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// parseDeptID 取路径参数里的部门主键，非法时已登记错误，返回 false 让调用方直接 return。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func parseDeptID(c *gin.Context) (int64, bool) {
	deptID, err := strconv.ParseInt(c.Param("deptId"), 10, 64)
	if err != nil || deptID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "部门主键不能为空", c.Param("deptId")))
		return 0, false
	}
	return deptID, true
}
