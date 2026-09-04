// Package handler 操作日志监控 HTTP 接口。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/monitor/service"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/excel"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
)

// OperLogApi 操作日志监控接口。
type OperLogApi struct{}

var OperLogApiApp = new(OperLogApi)

// List 分页查询操作日志列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *OperLogApi) List(c *gin.Context) {
	var q bo.SysOperLogQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := service.OperLogSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出操作日志列表为 xlsx 附件。
// 走 POST：前端 commonExport 以 form 表单提交筛选条件，故用 ShouldBind。
// 响应体是二进制附件，不返回 response.R。多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *OperLogApi) Export(c *gin.Context) {
	var q bo.SysOperLogQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := service.OperLogSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if err := excel.Export(c, rows, "操作日志"); err != nil {
		_ = c.Error(err)
		return
	}
}

// Remove 批量删除操作日志。主键串以逗号分隔。
func (a *OperLogApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("operIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数主键不能为空", c.Param("operIds")))
		return
	}
	if err := service.OperLogSvcApp.DeleteByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Clean 清空全部操作日志。
func (a *OperLogApi) Clean(c *gin.Context) {
	if err := service.OperLogSvcApp.Clean(c.Request.Context()); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
