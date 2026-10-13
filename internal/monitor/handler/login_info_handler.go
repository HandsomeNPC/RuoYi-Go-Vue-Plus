// Package handler 登录日志监控 HTTP 接口。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/excelhelper"
	pkgrepo "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/repository"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	logininfoservice "ruoyi-go-vue-plus/internal/monitor/service/logininfo"
	"ruoyi-go-vue-plus/internal/system/model/bo"

	"github.com/HandsomeNPC/ruoyi-common-go/strutil"
)

// LoginInfoApi 登录日志监控接口。
type LoginInfoApi struct{}

var LoginInfoApiApp = new(LoginInfoApi)

// List 分页查询登录日志列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *LoginInfoApi) List(c *gin.Context) {
	var q bo.SysLoginInfoQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := logininfoservice.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出登录日志列表为 xlsx 附件。
// 走 POST：前端 commonExport 以 form 表单提交筛选条件，故用 ShouldBind。
// 响应体是二进制附件，不返回 response.R。多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *LoginInfoApi) Export(c *gin.Context) {
	var q bo.SysLoginInfoQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := logininfoservice.QueryList(c.Request.Context(), q, excelhelper.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if err := excelhelper.Export(c, rows, "登录日志"); err != nil {
		reportExportErr(c, err)
		return
	}
}

// Remove 批量删除登录日志。主键串以逗号分隔。
func (a *LoginInfoApi) Remove(c *gin.Context) {
	ids, err := strutil.ParseIDList(c.Param("infoIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数主键不能为空", c.Param("infoIds")))
		return
	}
	if err := logininfoservice.DeleteByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Clean 清空全部登录日志。
func (a *LoginInfoApi) Clean(c *gin.Context) {
	if err := logininfoservice.Clean(c.Request.Context()); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Unlock 清除指定用户的登录失败锁定状态。userName 走路径参数，路径里不允许有斜杠。
func (a *LoginInfoApi) Unlock(c *gin.Context) {
	userName := c.Param("userName")
	if userName == "" {
		_ = c.Error(errs.New(response.CodeBadRequest, "用户名不能为空", ""))
		return
	}
	if err := logininfoservice.Unlock(c.Request.Context(), userName); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
