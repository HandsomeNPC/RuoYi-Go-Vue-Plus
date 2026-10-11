// Package handler 在线用户监控 HTTP 接口。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	pkgrepo "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/repository"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/satoken/loginhelper"
	onlineservice "ruoyi-go-vue-plus/internal/monitor/service/online"
)

// UserOnlineApi 在线用户监控接口。
type UserOnlineApi struct{}

var UserOnlineApiApp = new(UserOnlineApi)

// List 获取在线用户监控列表，按 IP 或用户名过滤当前有效会话。
func (a *UserOnlineApi) List(c *gin.Context) {
	rows, err := onlineservice.List(c.Request.Context(), c.Query("ipaddr"), c.Query("userName"))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(pkgrepo.PageOf(rows)))
}

// ForceLogout 按 token 强制用户下线。
func (a *UserOnlineApi) ForceLogout(c *gin.Context) {
	tokenID := c.Param("tokenId")
	if tokenID == "" {
		_ = c.Error(errs.New(response.CodeBadRequest, "token 不能为空", ""))
		return
	}
	if err := onlineservice.ForceLogout(c.Request.Context(), tokenID); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// GetInfo 获取当前登录用户的在线设备列表。
func (a *UserOnlineApi) GetInfo(c *gin.Context) {
	rows, err := onlineservice.GetInfo(c.Request.Context(), loginhelper.GetLoginID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(pkgrepo.PageOf(rows)))
}

// Remove 强退当前账号下指定在线设备，避免误踢其他账号会话。
func (a *UserOnlineApi) Remove(c *gin.Context) {
	tokenID := c.Param("tokenId")
	if tokenID == "" {
		_ = c.Error(errs.New(response.CodeBadRequest, "token 不能为空", ""))
		return
	}
	if err := onlineservice.Remove(c.Request.Context(), loginhelper.GetLoginID(c), tokenID); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
