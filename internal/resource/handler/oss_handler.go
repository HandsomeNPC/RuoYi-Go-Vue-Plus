package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/internal/resource/model/bo"
	"ruoyi-go-vue-plus/internal/resource/model/vo"
	resourceservice "ruoyi-go-vue-plus/internal/resource/service"
	"ruoyi-go-vue-plus/pkg/errs"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/response"
)

// maxUploadSize 单文件大小上限，对齐 Java spring.servlet.multipart.max-file-size。
const maxUploadSize = 10 << 20

// headerDownloadFilename 附件名响应头，与 pkg/excel 用的是同一个约定：
// 前端读它拿文件名，比解析 Content-Disposition 省事。
const headerDownloadFilename = "download-filename"

// OssApi 文件上传接口（对应 Java SysOssController）。
type OssApi struct{}

// OssApiApp 包级实例。
var OssApiApp = new(OssApi)

// List 分页查询 OSS 对象列表。
// 筛选条件与分页参数同在 query 上，分两次绑定同一份 URL 参数——query 绑定不消费 body，可重复调用。
func (a *OssApi) List(c *gin.Context) {
	var q bo.SysOssQueryBo
	if err := c.ShouldBindQuery(&q); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "参数校验失败", err.Error()))
		return
	}
	var page pkgrepo.PageQuery
	if err := c.ShouldBindQuery(&page); err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "分页参数有误", err.Error()))
		return
	}

	res, err := resourceservice.OssSvcApp.QueryPageList(c.Request.Context(), q, page)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(res))
}

// ListByIDs 按主键串查 OSS 对象（对应 Java listByIds）。
func (a *OssApi) ListByIDs(c *gin.Context) {
	ids, err := parseIDs(c.Param("ossIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("ossIds")))
		return
	}

	rows, err := resourceservice.OssSvcApp.ListByIDs(c.Request.Context(), ids)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.Ok(rows))
}

// Upload 上传文件（对应 Java upload）。
// 走 multipart/form-data：file 是文件本体，ossExt 是可选的扩展信息 JSON 串。
func (a *OssApi) Upload(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "上传文件不能为空", err.Error()))
		return
	}
	// 提前按声明大小拒绝，不等读完再判：省掉一次到对象存储的无用往返。
	if header.Size > maxUploadSize {
		_ = c.Error(errs.New(response.CodeBadRequest,
			fmt.Sprintf("上传文件不能超过 %dMB", maxUploadSize>>20), ""))
		return
	}

	item, err := resourceservice.OssSvcApp.Upload(c.Request.Context(), header, c.PostForm("ossExt"))
	if err != nil {
		_ = c.Error(err)
		return
	}

	c.JSON(http.StatusOK, response.Ok(&vo.SysOssUploadVo{
		URL:      item.URL,
		FileName: item.OriginalName,
		OssID:    item.OssID,
	}))
}

// Download 下载文件（对应 Java download）。
//
// 与导出接口同为不返回 response.R 的例外，响应体是二进制附件。
// 所有可能失败的活都得在写第一个字节之前干完：middleware.Recover 只在
// 响应未开写时才渲染错误，抢先写字节会让后续错误被静默吞掉。
func (a *OssApi) Download(c *gin.Context) {
	ossID, err := strconv.ParseInt(c.Param("ossId"), 10, 64)
	if err != nil || ossID <= 0 {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("ossId")))
		return
	}

	res, err := resourceservice.OssSvcApp.Download(c.Request.Context(), ossID)
	if err != nil {
		if errors.Is(err, resourceservice.ErrOssNotFound) {
			_ = c.Error(errs.New(response.CodeNotFound, "文件数据不存在!", ""))
			return
		}
		_ = c.Error(err)
		return
	}
	defer func() { _ = res.Close() }()

	contentType := res.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	encoded := percentEncode(res.OriginalName)

	h := c.Writer.Header()
	h.Set("Content-Type", contentType)
	h.Set("Content-Disposition", attachment(encoded))
	h.Set(headerDownloadFilename, encoded)
	if res.Size > 0 {
		h.Set("Content-Length", strconv.FormatInt(res.Size, 10))
	}
	// 附件名两个头都得放行到 CORS 之外，配置里漏了就地兜一层。
	h.Add("Access-Control-Expose-Headers", "Content-Disposition, "+headerDownloadFilename)

	c.Status(http.StatusOK)
	// 直通拷贝而非先读进内存：大文件全量缓冲会把进程打爆。
	if _, err := io.Copy(c.Writer, res.Body); err != nil {
		// 响应已开写，错误只能进日志：Recover 判定 Written() 后不再渲染，
		// 也不该渲染——那会把 JSON 拼到文件尾。
		_ = c.Error(fmt.Errorf("handler: 写文件响应失败: %w", err))
	}
}

// Remove 批量删除 OSS 对象（对应 Java remove）。
func (a *OssApi) Remove(c *gin.Context) {
	ids, err := parseIDs(c.Param("ossIds"))
	if err != nil {
		_ = c.Error(errs.New(response.CodeBadRequest, "主键不能为空", c.Param("ossIds")))
		return
	}

	if err := resourceservice.OssSvcApp.DeleteWithValidByIDs(c.Request.Context(), ids); err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OkVoid())
}

// attachment 拼 Content-Disposition。
//
// 同时给 filename 与 filename*：前者是纯 ASCII 兜底给老客户端，后者带 UTF-8 原名。
// 与 pkg/excel 的同名函数口径一致，只是这里 ASCII 兜底名也用编码值——
// 原始文件名没有 uuid 那样天然的 ASCII 替身。
func attachment(encoded string) string {
	return fmt.Sprintf("attachment; filename=%q; filename*=UTF-8''%s", encoded, encoded)
}

// percentEncode 百分号编码文件名，只放过 RFC 3986 的 unreserved 字符。
//
// 不用 url.QueryEscape（空格编成 +，而文件名里的 + 是字面加号，浏览器不会还原成空格），
// 也不用 url.PathEscape（它放过 = 和 @，二者都不是 RFC 5987 允许的 attr-char，
// 会把 filename* 的值截断）。与 pkg/excel 的同名实现一致，那个是包私有的取不到。
func percentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9',
			ch == '-', ch == '.', ch == '_', ch == '~':
			b.WriteByte(ch)
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
