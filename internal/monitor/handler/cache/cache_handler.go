// Package cache 缓存监控 HTTP 接口。
package cache

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	cacheservice "ruoyi-go-vue-plus/internal/monitor/service/cache"
)

// GetInfo 获取 Redis 缓存监控信息。
func GetInfo(c *gin.Context) {
	res, err := cacheservice.GetInfo(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
