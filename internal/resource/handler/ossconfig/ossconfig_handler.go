// Package ossconfig 对象存储配置 HTTP 接口。
package ossconfig

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	pkgpage "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/page"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/model/response"
	"ruoyi-go-vue-plus/internal/resource/model/bo"
	ossconfigservice "ruoyi-go-vue-plus/internal/resource/service/ossconfig"

	"github.com/HandsomeNPC/ruoyi-common-go/strutil"
)

// List 分页查询配置列表。
func List(c *gin.Context) {
	var q bo.SysOssConfigQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgpage.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := ossconfigservice.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// GetInfo 按主键查配置详情。
func GetInfo(c *gin.Context) {
	// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
	id, err := strconv.ParseInt(c.Param("ossConfigId"), 10, 64)
	if err != nil || id <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("ossConfigId")))
		return
	}

	item, err := ossconfigservice.QueryByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ossconfigservice.ErrOssConfigNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "对象存储配置不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(item))
}

// Add 新增配置。
func Add(c *gin.Context) {
	var b bo.SysOssConfigBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := ossconfigservice.InsertConfig(c.Request.Context(), &b); err != nil {
		if errors.Is(err, ossconfigservice.ErrOssConfigKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("操作配置'%s'失败, 配置key已存在!", b.ConfigKey), ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改配置。
func Edit(c *gin.Context) {
	var b bo.SysOssConfigBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysOssConfigBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.OssConfigID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", ""))
		return
	}

	if err := ossconfigservice.UpdateConfig(c.Request.Context(), &b); err != nil {
		if errors.Is(err, ossconfigservice.ErrOssConfigKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("操作配置'%s'失败, 配置key已存在!", b.ConfigKey), ""))
			return
		}
		if errors.Is(err, ossconfigservice.ErrOssConfigNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "对象存储配置不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// ChangeStatus 切换默认配置。
func ChangeStatus(c *gin.Context) {
	var b bo.SysOssConfigStatusBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := ossconfigservice.UpdateConfigStatus(c.Request.Context(), &b); err != nil {
		if errors.Is(err, ossconfigservice.ErrOssConfigNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "对象存储配置不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除配置。
func Remove(c *gin.Context) {
	ids, err := strutil.ParseIDList(c.Param("ossConfigIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("ossConfigIds")))
		return
	}

	if err := ossconfigservice.DeleteConfigs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
