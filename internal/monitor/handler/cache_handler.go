// Package handler 缓存监控 HTTP 接口。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/response"
	cacheservice "ruoyi-go-vue-plus/internal/monitor/service/cache"
)

// CacheApi 缓存监控接口。
type CacheApi struct{}

var CacheApiApp = new(CacheApi)

// GetInfo 获取 Redis 缓存监控信息。
func (a *CacheApi) GetInfo(c *gin.Context) {
	res, err := cacheservice.GetInfo(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
