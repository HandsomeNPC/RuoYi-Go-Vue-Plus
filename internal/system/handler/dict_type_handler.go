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

// DictTypeApi 字典类型接口。
type DictTypeApi struct{}

var DictTypeApiApp = new(DictTypeApi)

// List 分页查询字典类型列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *DictTypeApi) List(c *gin.Context) {
	var q bo.SysDictTypeQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.DictTypeSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出字典类型列表为 xlsx 附件。
// 走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件，
// 故用 ShouldBind 同时吃 form body 与 query。
//
// 响应体是二进制附件，不返回 response.R——见 pkg/excel 的说明。
// 多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *DictTypeApi) Export(c *gin.Context) {
	var q bo.SysDictTypeQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := systemservice.DictTypeSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "字典类型"); err != nil {
		_ = c.Error(err)
		return
	}
}

// GetInfo 按主键查字典类型详情。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func (a *DictTypeApi) GetInfo(c *gin.Context) {
	dictID, err := strconv.ParseInt(c.Param("dictId"), 10, 64)
	if err != nil || dictID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "字典主键不能为空", c.Param("dictId")))
		return
	}

	res, err := systemservice.DictTypeSvcApp.QueryByID(c.Request.Context(), dictID)
	if err != nil {
		if errors.Is(err, systemservice.ErrDictTypeNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "字典类型不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// OptionSelect 获取字典类型下拉选择列表。
// 不校验权限码，仅需登录：前端选字典时未必有字典管理权限。
func (a *DictTypeApi) OptionSelect(c *gin.Context) {
	res, err := systemservice.DictTypeSvcApp.QueryAll(c.Request.Context())
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增字典类型。
func (a *DictTypeApi) Add(c *gin.Context) {
	var b bo.SysDictTypeBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.DictTypeSvcApp.InsertDictType(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrDictTypeExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("新增字典'%s'失败，字典类型已存在", b.DictName), ""))
			return
		}
		// 字典类型命名格式不合规由 service 以 ServiceError 抛出，此处无需另行翻译。
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改字典类型。
func (a *DictTypeApi) Edit(c *gin.Context) {
	var b bo.SysDictTypeBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysDictTypeBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.DictID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "字典主键不能为空", ""))
		return
	}

	if err := systemservice.DictTypeSvcApp.UpdateDictType(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrDictTypeExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改字典'%s'失败，字典类型已存在", b.DictName), ""))
			return
		}
		if errors.Is(err, systemservice.ErrDictTypeNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "字典类型不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除字典类型。主键串以逗号分隔。
func (a *DictTypeApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("dictIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "字典主键不能为空", c.Param("dictIds")))
		return
	}

	// 「已分配,不能删除」的提示由 service 以 ServiceError 抛出，此处无需另行翻译。
	if err := systemservice.DictTypeSvcApp.DeleteDictTypeByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// RefreshCache 刷新字典缓存。
//
// 本实现只是清两组 Redis key
// （EvictGroup 的 SCAN+DEL 本身幂等），没有"清空后重新加载"的窗口期，并发清也不会
// 产生不一致。等将来改成预热式刷新再补锁。
func (a *DictTypeApi) RefreshCache(c *gin.Context) {
	if err := systemservice.DictTypeSvcApp.ResetDictCache(c.Request.Context()); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
