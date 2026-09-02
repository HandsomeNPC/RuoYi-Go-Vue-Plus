// Package handler 登录日志监控 HTTP 接口。
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/monitor/service"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/excel"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
)

// LoginInfoApi 登录日志监控接口（对应 Java SysLoginInfoController）。
type LoginInfoApi struct{}

// LoginInfoApiApp 包级实例。
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

	res, err := service.LoginInfoSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出登录日志列表为 xlsx 附件（对应 Java SysLoginInfoController.export）。
// 与 Java 一致走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件，故用 ShouldBind。
// 响应体是二进制附件，不返回 response.R。多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *LoginInfoApi) Export(c *gin.Context) {
	var q bo.SysLoginInfoQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := service.LoginInfoSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if err := excel.Export(c, rows, "登录日志"); err != nil {
		_ = c.Error(err)
		return
	}
}

// Remove 批量删除登录日志。主键串以逗号分隔，与 Java 的 @PathVariable Long[] infoIds 同形。
func (a *LoginInfoApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("infoIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数主键不能为空", c.Param("infoIds")))
		return
	}
	if err := service.LoginInfoSvcApp.DeleteByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Clean 清空全部登录日志。
func (a *LoginInfoApi) Clean(c *gin.Context) {
	if err := service.LoginInfoSvcApp.Clean(c.Request.Context()); err != nil {
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
	if err := service.LoginInfoSvcApp.Unlock(c.Request.Context(), userName); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// parseIDs 切分逗号分隔的主键串。任一段非法即整体拒绝——静默丢弃会删成部分成功。
// 与 internal/system/handler 的同名函数同实现：本包是独立的 monitor handler 包，取不到那个。
func parseIDs(raw string) ([]int64, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	for _, p := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("handler: 非法主键 %q", p)
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("handler: 主键为空")
	}
	return ids, nil
}
