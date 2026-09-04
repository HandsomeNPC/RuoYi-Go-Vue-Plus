package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
	"ruoyi-go-vue-plus/pkg/satoken/loginhelper"
)

// MessageApi 消息记录接口。
//
// 挂在 resource 模块因前端路径在 /resource/message 下；
// 业务逻辑仍归 system 的 MessageService——公告新增时也要发消息，
// 那条路径在 system 进程内，服务层搬过来反而要跨模块回调。
type MessageApi struct{}

var MessageApiApp = new(MessageApi)

// GetBox 查当前用户的消息盒子。
// 只按登录态取用户，不接受入参——否则任何人都能翻别人的消息。
func (a *MessageApi) GetBox(c *gin.Context) {
	userID := loginhelper.GetUserID(c)
	if userID == 0 {
		_ = c.Error(errs.New(response.CodeUnauthorized, "未登录", ""))
		return
	}

	res, err := systemservice.MessageSvcApp.QueryMessageBox(c.Request.Context(), userID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
