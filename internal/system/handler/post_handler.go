package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/excel"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
)

// PostApi 岗位信息接口。
type PostApi struct{}

var PostApiApp = new(PostApi)

// List 分页查询岗位列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *PostApi) List(c *gin.Context) {
	var q bo.SysPostQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.PostSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Export 导出岗位列表为 xlsx 附件。
// 走 POST：前端 commonExport 以 form 表单 POST 提交筛选条件，
// 故用 ShouldBind 同时吃 form body 与 query。
//
// 响应体是二进制附件，不返回 response.R——见 pkg/excel 的说明。
// 多取一行用于判定超限，避免"先捞完百万行再拒绝"。
func (a *PostApi) Export(c *gin.Context) {
	var q bo.SysPostQueryBo
	if err := c.ShouldBind(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	rows, err := systemservice.PostSvcApp.QueryList(c.Request.Context(), q, excel.MaxRows+1)
	if err != nil {
		_ = c.Error(err)
		return
	}

	// 工作簿在 excel.Export 内部先建满缓冲再落笔：
	// middleware.Recover 只在 !Written() 时渲染错误，抢先写字节会让后续错误被静默吞掉。
	if err := excel.Export(c, rows, "岗位数据"); err != nil {
		_ = c.Error(err)
		return
	}
}

// GetInfo 按主键查岗位详情。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func (a *PostApi) GetInfo(c *gin.Context) {
	postID, err := strconv.ParseInt(c.Param("postId"), 10, 64)
	if err != nil || postID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "岗位主键不能为空", c.Param("postId")))
		return
	}

	res, err := systemservice.PostSvcApp.QueryByID(c.Request.Context(), postID)
	if err != nil {
		if errors.Is(err, systemservice.ErrPostNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "岗位不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增岗位。
func (a *PostApi) Add(c *gin.Context) {
	var b bo.SysPostBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.PostSvcApp.InsertPost(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrPostNameExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("新增岗位'%s'失败，岗位名称已存在", b.PostName), ""))
			return
		}
		if errors.Is(err, systemservice.ErrPostCodeExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("新增岗位'%s'失败，岗位编码已存在", b.PostName), ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改岗位。
func (a *PostApi) Edit(c *gin.Context) {
	var b bo.SysPostBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysPostBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.PostID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "岗位主键不能为空", ""))
		return
	}

	if err := systemservice.PostSvcApp.UpdatePost(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrPostNameExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改岗位'%s'失败，岗位名称已存在", b.PostName), ""))
			return
		}
		if errors.Is(err, systemservice.ErrPostCodeExists) {
			_ = c.Error(errs.New(response.CodeFail,
				fmt.Sprintf("修改岗位'%s'失败，岗位编码已存在", b.PostName), ""))
			return
		}
		if errors.Is(err, systemservice.ErrPostHasUsers) {
			_ = c.Error(errs.New(response.CodeFail, "该岗位下存在已分配用户，不能禁用!", ""))
			return
		}
		if errors.Is(err, systemservice.ErrPostNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "岗位不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除岗位。主键串以逗号分隔。
func (a *PostApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("postIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "岗位主键不能为空", c.Param("postIds")))
		return
	}

	// 「已分配,不能删除」的提示由 service 以 ServiceError 抛出，此处无需另行翻译。
	if err := systemservice.PostSvcApp.DeletePostByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// OptionSelect 获取岗位选择框列表。
// deptId 与 postIds 均可缺省：传 deptId 返回该部门下全部岗位，
// 传 postIds 返回启用岗位，都不传返回全部启用岗位。
func (a *PostApi) OptionSelect(c *gin.Context) {
	deptID, _ := strconv.ParseInt(c.Query("deptId"), 10, 64)

	var postIDs []int64
	// 数组既可能以
	// postIds=1&postIds=2 下发，也可能是 postIds=1,2，两种都要吃下。
	if raw := strings.Join(c.QueryArray("postIds"), ","); strings.Trim(raw, ",") != "" {
		parsed, err := parseIDs(raw)
		if err != nil {
			_ = c.Error(errs.New(response.CodeBadRequest, "岗位主键有误", raw))
			return
		}
		postIDs = parsed
	}

	res, err := systemservice.PostSvcApp.OptionSelect(c.Request.Context(), deptID, postIDs)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// DeptTree 获取岗位筛选用的部门树。
func (a *PostApi) DeptTree(c *gin.Context) {
	var q bo.SysDeptQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	res, err := systemservice.DeptSvcApp.SelectDeptTreeList(c.Request.Context(), q)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}
