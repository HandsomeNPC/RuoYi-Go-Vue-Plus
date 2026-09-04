package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// SocialApi 社会化关系接口。
type SocialApi struct{}

var SocialApiApp = new(SocialApi)

// List 查询当前登录用户的社会化账号绑定列表。
// 不校验权限码，仅需登录：用户查自己的绑定关系不该卡权限。
func (a *SocialApi) List(c *gin.Context) {
	res, err := systemservice.SocialSvcApp.QueryListByUserId(
		c.Request.Context(), loginhelper.GetUserID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
