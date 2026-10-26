// Package message 消息记录 HTTP 接口。
package message

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/utils/loginhelper"
	messageservice "ruoyi-go-vue-plus/internal/system/service/message"
)

// GetBox 查当前用户的消息盒子。
// 只按登录态取用户，不接受入参——否则任何人都能翻别人的消息。
func GetBox(c *gin.Context) {
	userID := loginhelper.GetUserID(c)
	if userID == 0 {
		_ = c.Error(errs.New(response.CodeUnauthorized, "未登录", ""))
		return
	}

	res, err := messageservice.QueryMessageBox(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
