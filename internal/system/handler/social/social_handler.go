// Package social 社会化关系 HTTP 接口。
package social

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/utils/loginhelper"
	socialservice "ruoyi-go-vue-plus/internal/system/service/social"
)

// List 查询当前登录用户的社会化账号绑定列表。
// 不校验权限码，仅需登录：用户查自己的绑定关系不该卡权限。
func List(c *gin.Context) {
	res, err := socialservice.QueryListByUserId(
		c.Request.Context(), loginhelper.GetUserID(c))
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
