package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	systemservice "ruoyi-go-vue-plus/internal/system/service"
	"ruoyi-go-vue-plus/pkg/errs"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
)

// NoticeApi 通知公告接口（对应 Java SysNoticeController）。
type NoticeApi struct{}

// NoticeApiApp 包级实例。
var NoticeApiApp = new(NoticeApi)

// List 分页查询通知公告列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *NoticeApi) List(c *gin.Context) {
	var q bo.SysNoticeQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := systemservice.NoticeSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// GetInfo 按主键查通知公告详情。
// 主键走路径参数，超出 JS 安全整数的 ID 前端会以字符串下发，ParseInt 两种形态都吃得下。
func (a *NoticeApi) GetInfo(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("noticeId"), 10, 64)
	if err != nil || id <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "公告主键不能为空", c.Param("noticeId")))
		return
	}

	res, err := systemservice.NoticeSvcApp.QueryByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, systemservice.ErrNoticeNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "通知公告不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// Add 新增通知公告。新增成功后由 service 向在线用户广播公告摘要。
func (a *NoticeApi) Add(c *gin.Context) {
	var b bo.SysNoticeBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}

	if err := systemservice.NoticeSvcApp.InsertNotice(c.Request.Context(), &b); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Edit 修改通知公告。
func (a *NoticeApi) Edit(c *gin.Context) {
	var b bo.SysNoticeBo
	if err := c.ShouldBindJSON(&b); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	// 主键校验单独做：SysNoticeBo 与新增共用，加 binding:"required" 会连带卡住新增。
	if b.NoticeID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "公告主键不能为空", ""))
		return
	}

	if err := systemservice.NoticeSvcApp.UpdateNotice(c.Request.Context(), &b); err != nil {
		if errors.Is(err, systemservice.ErrNoticeNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "通知公告不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// Remove 批量删除通知公告。主键串以逗号分隔，与 Java 的 @PathVariable Long[] noticeIds 同形。
func (a *NoticeApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("noticeIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "公告主键不能为空", c.Param("noticeIds")))
		return
	}

	if err := systemservice.NoticeSvcApp.DeleteNoticeByIDs(c.Request.Context(), ids); err != nil {
		if errors.Is(err, systemservice.ErrNoticeNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "通知公告不存在", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}
