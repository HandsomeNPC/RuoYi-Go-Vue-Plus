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
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
)

// ClientApi 客户端管理接口（对应 Java SysClientController）。
type ClientApi struct{}

// ClientApiApp 包级实例。
var ClientApiApp = new(ClientApi)

// List 分页查询客户端列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *ClientApi) List(c *gin.Context) {
	var q bo.SysClientQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.ClientSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// GetInfo 按主键查客户端详情。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func (a *ClientApi) GetInfo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("id")))
		return
	}

	res, err := systemservice.ClientSvcApp.QueryByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, systemservice.ErrClientNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "客户端不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增客户端。
func (a *ClientApi) Add(c *gin.Context) {
	var b bo.SysClientBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.ClientSvcApp.InsertByBo(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrClientKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("新增客户端'%s'失败，客户端key已存在", b.ClientKey), ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改客户端。
func (a *ClientApi) Edit(c *gin.Context) {
	var b bo.SysClientBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysClientBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.ID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "id不能为空", ""))
		return
	}

	if err := systemservice.ClientSvcApp.UpdateByBo(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrClientKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改客户端'%s'失败，客户端key已存在", b.ClientKey), ""))
			return
		}
		if errors.Is(err, systemservice.ErrClientNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "客户端不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// ChangeStatus 修改客户端启停状态。
func (a *ClientApi) ChangeStatus(c *gin.Context) {
	var b bo.SysClientStatusBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	err := systemservice.ClientSvcApp.UpdateClientStatus(c.Request.Context(), b.ClientID, b.Status)
	if err != nil {
		if errors.Is(err, systemservice.ErrClientNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "客户端不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
