// Package handler 缓存监控 HTTP 接口。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/monitor/service"
	"ruoyi-go-vue-plus/pkg/response"
)

// CacheApi 缓存监控接口（对应 Java CacheController）。
type CacheApi struct{}

// CacheApiApp 包级实例。
var CacheApiApp = new(CacheApi)

// GetInfo 获取 Redis 缓存监控信息（对应 Java CacheController.getInfo）。
func (a *CacheApi) GetInfo(c *gin.Context) {
	res, err := service.CacheSvcApp.GetInfo(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
