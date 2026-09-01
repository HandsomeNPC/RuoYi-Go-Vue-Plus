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

// DictDataApi 字典数据接口（对应 Java SysDictDataController）。
type DictDataApi struct{}

// DictDataApiApp 包级实例。
var DictDataApiApp = new(DictDataApi)

// List 分页查询字典数据列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *DictDataApi) List(c *gin.Context) {
	var q bo.SysDictDataQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.DictDataSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出字典数据列表为 xlsx 附件（对应 Java SysDictDataController.export）。
// 与 Java 一致走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件，
// 故用 ShouldBind 同时吃 form body 与 query。
//
// 响应体是二进制附件，不返回 response.R——见 pkg/excel 的说明。
// 多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *DictDataApi) Export(c *gin.Context) {
	var q bo.SysDictDataQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := systemservice.DictDataSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "字典数据"); err != nil {
		_ = c.Error(err)
		return
	}
}

// GetInfo 按字典编码查字典数据详情。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func (a *DictDataApi) GetInfo(c *gin.Context) {
	dictCode, err := strconv.ParseInt(c.Param("dictCode"), 10, 64)
	if err != nil || dictCode <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "字典编码不能为空", c.Param("dictCode")))
		return
	}

	res, err := systemservice.DictDataSvcApp.QueryByID(c.Request.Context(), dictCode)
	if err != nil {
		if errors.Is(err, systemservice.ErrDictDataNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "字典数据不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// DictType 按字典类型查字典数据列表（对应 Java dictType）。
//
// 与 Java 一致不校验权限码，仅需登录：前端 DictTag 组件到处要渲染字典标签，
// 却未必有字典管理权限。类型不存在时返回 200 + 空数组（service 已按空切片兜底）。
func (a *DictDataApi) DictType(c *gin.Context) {
	dictType := c.Param("dictType")
	if dictType == "" {
		_ = c.Error(errs.New(response.CodeBadRequest, "字典类型不能为空", ""))
		return
	}

	res, err := systemservice.DictTypeSvcApp.SelectDictDataByType(c.Request.Context(), dictType)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增字典数据。
func (a *DictDataApi) Add(c *gin.Context) {
	var b bo.SysDictDataBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.DictDataSvcApp.InsertDictData(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrDictDataValueExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("新增字典数据'%s'失败，字典键值已存在", b.DictValue), ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改字典数据。
func (a *DictDataApi) Edit(c *gin.Context) {
	var b bo.SysDictDataBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysDictDataBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.DictCode <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "字典编码不能为空", ""))
		return
	}

	if err := systemservice.DictDataSvcApp.UpdateDictData(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrDictDataValueExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改字典数据'%s'失败，字典键值已存在", b.DictValue), ""))
			return
		}
		if errors.Is(err, systemservice.ErrDictDataNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "字典数据不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除字典数据。主键串以逗号分隔，与 Java 的 @PathVariable Long[] dictCodes 同形。
func (a *DictDataApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("dictCodes"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "字典编码不能为空", c.Param("dictCodes")))
		return
	}

	if err := systemservice.DictDataSvcApp.DeleteDictDataByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
