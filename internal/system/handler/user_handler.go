package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	systemvo "ruoyi-go-vue-plus/internal/system/model/vo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// UserApi 用户信息接口（对应 Java SysUserController）。
type UserApi struct{}

// UserApiApp 包级实例。
var UserApiApp = new(UserApi)

// GetInfo 获取当前登录用户信息、角色与权限集合（对照 Java SysUserController.getInfo）。
// 与 Java 一致：不校验权限码，仅需登录；user 查不到时返回失败。
func (a *UserApi) GetInfo(c *gin.Context) {
	loginUser := loginhelper.GetLoginUser(c)
	if loginUser == nil || loginUser.UserID == 0 {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	user, err := systemservice.UserSvcApp.SelectUserByID(c.Request.Context(), loginUser.UserID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if user == nil {
		c.JSON(http.StatusOK, response.Fail("没有权限访问用户数据!"))
		return
	}
	// permissions/roles 复用登录时存入 token session 的快照，避免每次拉菜单表。
	c.JSON(http.StatusOK, response.Ok(systemvo.UserInfoVo{
		User:        *user,
		Permissions: loginUser.MenuPermission,
		Roles:       loginUser.RolePermission,
	}))
}
