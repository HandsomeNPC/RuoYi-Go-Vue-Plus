package service

import (
	"context"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"path"
	"strconv"
	"time"

	"ruoyi-go-vue-plus/internal/resource/model"
	"ruoyi-go-vue-plus/internal/resource/model/bo"
	"ruoyi-go-vue-plus/internal/resource/model/vo"
	"ruoyi-go-vue-plus/internal/resource/repository"
	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/jsonx"
	"ruoyi-go-vue-plus/pkg/oss"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

var ErrOssNotFound = errors.New("service: 文件数据不存在")

// presignTTL 私有桶预签名链接的有效期。
const presignTTL = 120 * time.Second

// OssService OSS 对象存储业务逻辑。
type OssService struct{}

var OssSvcApp = new(OssService)

// DownloadResult 下载所需的元信息与数据流，调用方须关闭 Body。
type DownloadResult struct {
	// OriginalName 原始文件名，用于拼 Content-Disposition。
	OriginalName string
	ContentType  string
	Size         int64
	// Body 对象数据流，直通响应，不在此处读进内存
	// （全量缓冲会把大文件的进程内存打爆）。
	Body io.ReadCloser
}

// Close 释放底层连接。
func (d *DownloadResult) Close() error {
	if d.Body == nil {
		return nil
	}
	return d.Body.Close()
}

// QueryPageList 按条件分页查 OSS 对象，私有桶的 URL 换成预签名链接。
func (s *OssService) QueryPageList(ctx context.Context, q bo.SysOssQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysOssVo], error) {

	res, err := repository.NewOssRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysOssVo](), err
	}

	rows := vo.Conv.ConvertToSysOssVoList(res.Rows)
	for _, item := range rows {
		s.matchingURL(ctx, item)
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建。
	return pkgrepo.Page(rows, res.Total), nil
}

// ListByIDs 按主键批量查 OSS 对象，缺失的主键静默跳过。
func (s *OssService) ListByIDs(ctx context.Context, ids []int64) ([]*vo.SysOssVo, error) {
	out := make([]*vo.SysOssVo, 0, len(ids))
	for _, id := range ids {
		item, err := s.getByID(ctx, id)
		if err != nil {
			if errors.Is(err, ErrOssNotFound) {
				continue
			}
			return nil, err
		}
		s.matchingURL(ctx, item)
		out = append(out, item)
	}
	return out, nil
}

// getByID 读穿缓存取单条。
//
// 缓存的是库里的原始记录，不含预签名 URL：那个链接 120 秒就过期，
// 缓存进去（TTL 30 天）等于发一批必然失效的地址。
func (s *OssService) getByID(ctx context.Context, ossID int64) (*vo.SysOssVo, error) {
	var item vo.SysOssVo
	err := cache.GetOrSet(ctx, constant.CacheSysOss, ossKey(ossID), constant.CacheTTLSysOss,
		&item, func(ctx context.Context) (any, error) {
			row, err := repository.NewOssRepository(database.DB()).SelectByID(ctx, ossID)
			if err != nil {
				if errors.Is(err, repository.ErrOssNotFound) {
					return nil, ErrOssNotFound
				}
				return nil, err
			}
			return vo.Conv.ConvertToSysOssVo(row), nil
		})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// ossKey 缓存键。
func ossKey(ossID int64) string {
	return strconv.FormatInt(ossID, 10)
}

// matchingURL 私有桶把访问地址换成限时预签名链接。
//
// 取客户端或签名失败只记日志、保留库里的原地址：某个配置的服务不可达时，
// 整页列表仍应能打开（Java 的 queryPageList 未做此保护，一处不可达整页报错，
// 而它自己的 listByIds 又做了——此处统一按降级处理）。
func (s *OssService) matchingURL(ctx context.Context, item *vo.SysOssVo) {
	if item == nil {
		return
	}
	client, err := oss.Instance(ctx, item.Service)
	if err != nil {
		log.Printf("[oss] 取配置 %q 的客户端失败，沿用库中地址: %v", item.Service, err)
		return
	}
	if !client.IsPrivate() {
		return
	}
	url, err := client.PresignGetURL(ctx, item.FileName, presignTTL)
	if err != nil {
		log.Printf("[oss] 生成 %q 的预签名地址失败，沿用库中地址: %v", item.FileName, err)
		return
	}
	item.URL = url
}

// Upload 上传文件并落库。
// ossExtJSON 是前端可选带上的扩展信息 JSON 串，解析失败按空处理。
func (s *OssService) Upload(ctx context.Context, header *multipart.FileHeader,
	ossExtJSON string) (*vo.SysOssVo, error) {

	if header == nil || header.Size <= 0 {
		return nil, errs.New(0, "上传文件不能为空", "")
	}

	client, err := oss.InstanceDefault(ctx)
	if err != nil {
		return nil, err
	}

	src, err := header.Open()
	if err != nil {
		return nil, errs.New(0, "读取上传文件失败", err.Error())
	}
	defer func() { _ = src.Close() }()

	originalName := path.Base(header.Filename)
	contentType := header.Header.Get("Content-Type")
	key := client.BuildPathKey(originalName)

	result, err := client.Upload(ctx, key, src, header.Size, contentType)
	if err != nil {
		return nil, err
	}

	add := &model.SysOss{
		OssID:        snowflake.Next(), // oss_id 无 auto_increment
		FileName:     result.Key,
		OriginalName: originalName,
		// 后缀含点；无扩展名时为空串（Java 那边会造出个假后缀，此处不复刻）。
		FileSuffix: path.Ext(originalName),
		URL:        result.URL,
		Ext1:       buildOssExt(ossExtJSON, header.Size, contentType),
		// 存 configKey 而非厂商名：默认配置切换后，老文件仍按上传时的配置下载。
		Service: client.ConfigKey(),
	}
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewOssRepository(database.DB()).Insert(ctx, add); err != nil {
		return nil, err
	}

	item := vo.Conv.ConvertToSysOssVo(add)
	s.matchingURL(ctx, item)
	return item, nil
}

// buildOssExt 合并前端扩展信息与服务端实测的大小/类型，序列化成 ext1 的 JSON。
func buildOssExt(ossExtJSON string, size int64, contentType string) string {
	ext := new(model.SysOssExt)
	if ossExtJSON != "" {
		if err := jsonx.Unmarshal([]byte(ossExtJSON), ext); err != nil {
			// 扩展信息是附加描述，坏了不该让上传失败——文件已经传上去了。
			log.Printf("[oss] 解析 ossExt 失败，按空处理: %v", err)
			ext = new(model.SysOssExt)
		}
	}
	// 大小与类型以服务端实测为准，不信前端上报的值。
	ext.FileSize = size
	ext.ContentType = contentType

	b, err := jsonx.Marshal(ext)
	if err != nil {
		log.Printf("[oss] 序列化 ossExt 失败，ext1 留空: %v", err)
		return ""
	}
	return string(b)
}

// Download 取文件流。调用方须关闭返回的 DownloadResult。
func (s *OssService) Download(ctx context.Context, ossID int64) (*DownloadResult, error) {
	row, err := repository.NewOssRepository(database.DB()).SelectByID(ctx, ossID)
	if err != nil {
		if errors.Is(err, repository.ErrOssNotFound) {
			return nil, ErrOssNotFound
		}
		return nil, err
	}

	client, err := oss.Instance(ctx, row.Service)
	if err != nil {
		return nil, err
	}
	meta, body, err := client.Download(ctx, row.FileName)
	if err != nil {
		return nil, err
	}

	return &DownloadResult{
		OriginalName: row.OriginalName,
		ContentType:  meta.ContentType,
		Size:         meta.Size,
		Body:         body,
	}, nil
}

// DeleteWithValidByIDs 批量删除：先删远端对象再删库记录，最后失效缓存。
//
// 远端删除失败只记日志不中断：对象可能已被人工清理，卡在这里会让库里
// 留下一条永远删不掉的记录。
func (s *OssService) DeleteWithValidByIDs(ctx context.Context, ids []int64) error {
	repo := repository.NewOssRepository(database.DB())
	rows, err := repo.SelectByIDs(ctx, ids)
	if err != nil {
		return err
	}

	for _, row := range rows {
		client, err := oss.Instance(ctx, row.Service)
		if err != nil {
			log.Printf("[oss] 取配置 %q 的客户端失败，跳过远端删除: %v", row.Service, err)
			continue
		}
		if err := client.Delete(ctx, row.FileName); err != nil {
			log.Printf("[oss] 删除远端对象 %q 失败: %v", row.FileName, err)
		}
	}

	if _, err := repo.DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	// 删除后再失效，且必须失效：那份缓存 TTL 30 天，不清的话
	// listByIds 会一直返回已删文件。
	for _, row := range rows {
		_ = cache.Evict(ctx, constant.CacheSysOss, ossKey(row.OssID))
	}
	return nil
}
