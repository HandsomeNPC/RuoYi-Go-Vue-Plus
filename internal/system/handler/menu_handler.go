package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// MenuApi 菜单信息接口（对应 Java SysMenuController）。
type MenuApi struct{}

// MenuApiApp 包级实例。
var MenuApiApp = new(MenuApi)

// GetRouters 获取当前用户可访问的路由信息（对照 Java SysMenuController.getRouters）。
func (a *MenuApi) GetRouters(c *gin.Context) {
	menus, err := systemservice.MenuSvcApp.SelectMenuTreeByUserId(
		c.Request.Context(), loginhelper.GetUserID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(systemservice.MenuSvcApp.BuildMenus(menus)))
}
