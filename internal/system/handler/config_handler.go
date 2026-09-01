package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/excel"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
)

// ConfigApi 参数配置接口（对应 Java SysConfigController）。
type ConfigApi struct{}

// ConfigApiApp 包级实例。
var ConfigApiApp = new(ConfigApi)

// List 分页查询参数配置列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *ConfigApi) List(c *gin.Context) {
	var q bo.SysConfigQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.ConfigSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出参数配置列表为 xlsx 附件（对应 Java SysConfigController.export）。
// 与 Java 一致走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件，
// 故用 ShouldBind 同时吃 form body 与 query。
//
// 响应体是二进制附件，不返回 response.R——见 pkg/excel 的说明。
// 多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *ConfigApi) Export(c *gin.Context) {
	var q bo.SysConfigQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := systemservice.ConfigSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "参数数据"); err != nil {
		_ = c.Error(err)
		return
	}
}

// GetInfo 按主键查参数配置详情。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func (a *ConfigApi) GetInfo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("configId"), 10, 64)
	if err != nil || id <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数主键不能为空", c.Param("configId")))
		return
	}

	res, err := systemservice.ConfigSvcApp.QueryByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, systemservice.ErrConfigNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "参数配置不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// GetConfigKey 按参数键名查参数值（对应 Java getConfigKey）。
//
// 与 Java 一致不校验权限码，仅需登录：前端多处要读参数（如初始密码提示）却未必有配置管理权限。
// 键不存在时同样返回 200 + 空串（service 已按空串兜底），前端据此走默认值分支。
func (a *ConfigApi) GetConfigKey(c *gin.Context) {
	configKey := c.Param("configKey")
	if configKey == "" {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数键名不能为空", ""))
		return
	}

	value, err := systemservice.ConfigSvcApp.SelectConfigByKey(c.Request.Context(), configKey)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(value))
}

// Add 新增参数配置。
func (a *ConfigApi) Add(c *gin.Context) {
	var b bo.SysConfigBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.ConfigSvcApp.InsertConfig(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrConfigKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("新增参数'%s'失败，参数键名已存在", b.ConfigName), ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改参数配置。
func (a *ConfigApi) Edit(c *gin.Context) {
	var b bo.SysConfigBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysConfigBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.ConfigID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数主键不能为空", ""))
		return
	}

	if err := systemservice.ConfigSvcApp.UpdateConfig(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrConfigKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改参数'%s'失败，参数键名已存在", b.ConfigName), ""))
			return
		}
		if errors.Is(err, systemservice.ErrConfigNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "参数配置不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// UpdateByKey 按参数键名修改参数值（对应 Java updateByKey）。
func (a *ConfigApi) UpdateByKey(c *gin.Context) {
	var b bo.SysConfigUpdateByKeyBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.ConfigSvcApp.UpdateConfigByKey(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrConfigNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "参数配置不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除参数配置。主键串以逗号分隔，与 Java 的 @PathVariable Long[] configIds 同形。
func (a *ConfigApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("configIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数主键不能为空", c.Param("configIds")))
		return
	}

	// 内置参数不可删的提示由 service 以 ServiceError 抛出，此处无需另行翻译。
	if err := systemservice.ConfigSvcApp.DeleteConfigByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// RefreshCache 刷新参数缓存。
func (a *ConfigApi) RefreshCache(c *gin.Context) {
	if err := systemservice.ConfigSvcApp.ResetConfigCache(c.Request.Context()); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
