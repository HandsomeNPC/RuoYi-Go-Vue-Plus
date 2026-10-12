package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/errs"
	"github.com/HandsomeNPC/ruoyi-common-go/excel"
	pkgrepo "github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/repository"
	"github.com/HandsomeNPC/ruoyi-common-go/ruoyi-core/response"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	clientservice "ruoyi-go-vue-plus/internal/system/service/client"

	"github.com/HandsomeNPC/ruoyi-common-go/strutil"
)

// ClientApi 客户端管理接口。
type ClientApi struct{}

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

	res, err := clientservice.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出客户端列表为 xlsx 附件。
// 走 POST：前端 download() 与 commonExport 都以 form 表单 POST 提交筛选条件，
// 故用 ShouldBind 同时吃 form body 与 query。
//
// 本项目唯一不返回 response.R 的 handler，响应体是二进制附件。
// 多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *ClientApi) Export(c *gin.Context) {
	var q bo.SysClientQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := clientservice.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "客户端管理"); err != nil {
		reportExportErr(c, err)
		return
	}
}

// GetInfo 按主键查客户端详情。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func (a *ClientApi) GetInfo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("id")))
		return
	}

	res, err := clientservice.QueryByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, clientservice.ErrClientNotFound) {
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

	if err := clientservice.InsertByBo(c.Request.Context(), &b); err != nil {
		if errors.Is(err, clientservice.ErrClientKeyExists) {
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

	if err := clientservice.UpdateByBo(c.Request.Context(), &b); err != nil {
		if errors.Is(err, clientservice.ErrClientKeyExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改客户端'%s'失败，客户端key已存在", b.ClientKey), ""))
			return
		}
		if errors.Is(err, clientservice.ErrClientNotFound) {
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

	err := clientservice.UpdateClientStatus(c.Request.Context(), b.ClientID, b.Status)
	if err != nil {
		if errors.Is(err, clientservice.ErrClientNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "客户端不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除客户端。主键串以逗号分隔。
func (a *ClientApi) Remove(c *gin.Context) {
	ids, err := strutil.ParseIDList(c.Param("ids"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("ids")))
		return
	}

	if err := clientservice.DeleteWithValidByIDs(c.Request.Context(), ids); err != nil {
		if errors.Is(err, clientservice.ErrClientNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "客户端不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
