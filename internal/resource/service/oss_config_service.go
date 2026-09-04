package service

import (
	"context"
	"errors"
	"log"
	"strconv"

	"ruoyi-go-vue-plus/internal/resource/model/bo"
	"ruoyi-go-vue-plus/internal/resource/model/vo"
	"ruoyi-go-vue-plus/internal/resource/repository"
	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/oss"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

var ErrOssConfigNotFound = errors.New("service: 对象存储配置不存在")

var ErrOssConfigKeyExists = errors.New("service: 对象存储配置键已存在")

// systemConfigIDs 系统内置配置主键，不允许删除。
// 只含前四条（minio/qiniu/aliyun/qcloud）；seed 里的 image 配置有意不在其中，可删。
var systemConfigIDs = map[int64]struct{}{
	1761900000000000001: {},
	1761900000000000002: {},
	1761900000000000003: {},
	1761900000000000004: {},
}

// OssConfigService 对象存储配置业务逻辑。
type OssConfigService struct{}

var OssConfigSvcApp = new(OssConfigService)

// InitCache 启动时把全部配置写进缓存，并确定默认配置键。
//
// 必须在进程启动时跑一次：上传走 oss.InstanceDefault，它只认缓存与
// OssDefaultConfigKey，不预热的话第一次上传直接失败。
func (s *OssConfigService) InitCache(ctx context.Context) error {
	rows, err := repository.NewOssConfigRepository(database.DB()).SelectAll(ctx)
	if err != nil {
		return err
	}

	for _, cfg := range rows {
		if err := cache.Put(ctx, constant.CacheSysOssConfig, cfg.ConfigKey, cfg,
			constant.CacheTTLSysOssConfig); err != nil {
			// 预热失败不阻断启动：Redis 恢复后改一次配置即可回填，
			// 而这里 panic 会让整个进程起不来。
			log.Printf("[oss] 预热配置 %q 失败: %v", cfg.ConfigKey, err)
			continue
		}
		if cfg.Status == constant.Yes {
			s.setDefaultConfigKey(ctx, cfg.ConfigKey)
		}
	}
	return nil
}

// setDefaultConfigKey 记录当前生效的配置键。
func (s *OssConfigService) setDefaultConfigKey(ctx context.Context, configKey string) {
	if err := pkgredis.Client().Set(ctx, constant.OssDefaultConfigKey, configKey, 0).Err(); err != nil {
		log.Printf("[oss] 写默认配置键 %q 失败: %v", configKey, err)
	}
}

// QueryPageList 按条件分页查配置。
func (s *OssConfigService) QueryPageList(ctx context.Context, q bo.SysOssConfigQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysOssConfigVo], error) {

	res, err := repository.NewOssConfigRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysOssConfigVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建。
	return pkgrepo.Page(vo.Conv.ConvertToSysOssConfigVoList(res.Rows), res.Total), nil
}

// QueryByID 按主键查配置，不存在时返回 ErrOssConfigNotFound。
func (s *OssConfigService) QueryByID(ctx context.Context,
	ossConfigID int64) (*vo.SysOssConfigVo, error) {

	cfg, err := repository.NewOssConfigRepository(database.DB()).SelectByID(ctx, ossConfigID)
	if err != nil {
		if errors.Is(err, repository.ErrOssConfigNotFound) {
			return nil, ErrOssConfigNotFound
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysOssConfigVo(cfg), nil
}

// CheckConfigKeyUnique 校验配置键是否可用（唯一即 true）。
// excludeID > 0 时排除该主键，供修改场景排除自身。
func (s *OssConfigService) CheckConfigKeyUnique(ctx context.Context, configKey string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewOssConfigRepository(database.DB()).
		ExistsByConfigKey(ctx, configKey, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// InsertConfig 新增配置。配置键重复时返回 ErrOssConfigKeyExists。
func (s *OssConfigService) InsertConfig(ctx context.Context, b *bo.SysOssConfigBo) error {
	if b == nil {
		return errors.New("service: 对象存储配置入参为空")
	}

	unique, err := s.CheckConfigKeyUnique(ctx, b.ConfigKey, 0) // 新增无自身可排除
	if err != nil {
		return err
	}
	if !unique {
		return ErrOssConfigKeyExists
	}

	add := bo.Conv.ConvertToSysOssConfig(b)
	add.OssConfigID = snowflake.Next() // oss_config_id 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewOssConfigRepository(database.DB()).Insert(ctx, add); err != nil {
		return err
	}
	b.OssConfigID = add.OssConfigID
	s.refreshConfigCache(ctx, add.OssConfigID, add.ConfigKey, "")
	return nil
}

// UpdateConfig 按主键修改配置。
// 配置键被别的配置占用时返回 ErrOssConfigKeyExists；主键不存在返回 ErrOssConfigNotFound。
func (s *OssConfigService) UpdateConfig(ctx context.Context, b *bo.SysOssConfigBo) error {
	if b == nil {
		return errors.New("service: 对象存储配置入参为空")
	}
	if b.OssConfigID <= 0 {
		return errors.New("service: 对象存储配置主键不能为空")
	}

	unique, err := s.CheckConfigKeyUnique(ctx, b.ConfigKey, b.OssConfigID) // 改回原键不算冲突
	if err != nil {
		return err
	}
	if !unique {
		return ErrOssConfigKeyExists
	}

	repo := repository.NewOssConfigRepository(database.DB())
	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行，会把一次幂等的重复保存误报成"配置不存在"。
	// 顺带拿到旧配置键，改名时要清掉旧缓存。
	old, err := repo.SelectByID(ctx, b.OssConfigID)
	if err != nil {
		if errors.Is(err, repository.ErrOssConfigNotFound) {
			return ErrOssConfigNotFound
		}
		return err
	}

	if _, err := repo.UpdateByID(ctx, b.OssConfigID, buildOssConfigUpdateColumns(b)); err != nil {
		return err
	}
	s.refreshConfigCache(ctx, b.OssConfigID, b.ConfigKey, old.ConfigKey)
	return nil
}

// buildOssConfigUpdateColumns 组装更新列。
//
// 取舍依据是「该字段被清空是否为用户的合法意图」：
// 前缀/自定义域名/区域/扩展/备注是可选项，一律写入才能让编辑表单把它们清空；
// 状态/桶权限/https 是控制字段，缺省即视为不改——漏传不该把默认配置刷成非默认。
func buildOssConfigUpdateColumns(b *bo.SysOssConfigBo) map[string]any {
	columns := map[string]any{
		"config_key":  b.ConfigKey,
		"access_key":  b.AccessKey,
		"secret_key":  b.SecretKey,
		"bucket_name": b.BucketName,
		"endpoint":    b.Endpoint,
		"prefix":      b.Prefix,
		"domain_url":  b.DomainURL,
		"region":      b.Region,
		"ext1":        b.Ext1,
		"remark":      b.Remark,
	}
	if b.IsHttps != "" {
		columns["is_https"] = b.IsHttps
	}
	if b.Status != "" {
		columns["status"] = b.Status
	}
	if b.AccessPolicy != "" {
		columns["access_policy"] = b.AccessPolicy
	}
	return columns
}

// refreshConfigCache 写入配置缓存，并在配置键改名时清掉旧键。
//
// 从库里回读而非直接用 BO：更新走的是部分列，BO 里缺省的控制字段没被改，
// 直接缓存 BO 会把库里的真实值覆盖成空。
func (s *OssConfigService) refreshConfigCache(ctx context.Context, ossConfigID int64,
	configKey, oldConfigKey string) {

	// 改名时旧键的缓存与客户端都得清：写新键只是新增一份，旧的会成孤儿，
	// 而 sys_oss.service 里记着旧键的老文件仍要能按旧配置下载则另说——
	// 那种情况下老文件本就该随配置改名一起失效。
	if oldConfigKey != "" && oldConfigKey != configKey {
		_ = cache.Evict(ctx, constant.CacheSysOssConfig, oldConfigKey)
		oss.Remove(oldConfigKey)
	}

	cfg, err := repository.NewOssConfigRepository(database.DB()).SelectByID(ctx, ossConfigID)
	if err != nil {
		log.Printf("[oss] 回读配置 id=%d 失败，缓存未刷新: %v", ossConfigID, err)
		return
	}
	_ = cache.Put(ctx, constant.CacheSysOssConfig, cfg.ConfigKey, cfg,
		constant.CacheTTLSysOssConfig)
	// 客户端持有旧配置建的连接，主动丢弃让下次取用时重建。
	oss.Remove(cfg.ConfigKey)
	if cfg.Status == constant.Yes {
		s.setDefaultConfigKey(ctx, cfg.ConfigKey)
	}
}

// DeleteConfigs 批量删除配置，内置配置不可删。
func (s *OssConfigService) DeleteConfigs(ctx context.Context, ids []int64) error {
	repo := repository.NewOssConfigRepository(database.DB())
	rows, err := repo.SelectByIDs(ctx, ids)
	if err != nil {
		return err
	}

	// 先整批校验再删，不边删边校验：这里没有事务包裹，
	// 一旦先删了几行再撞上内置配置就会留下删一半的状态。
	for _, cfg := range rows {
		if _, ok := systemConfigIDs[cfg.OssConfigID]; ok {
			return errs.New(0, "系统内置, 不可删除!", strconv.FormatInt(cfg.OssConfigID, 10))
		}
	}

	if _, err := repo.DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	// 删除后再失效：提前清会让删除失败时白丢一批热配置。
	for _, cfg := range rows {
		_ = cache.Evict(ctx, constant.CacheSysOssConfig, cfg.ConfigKey)
		oss.Remove(cfg.ConfigKey)
	}
	return nil
}

// UpdateConfigStatus 切换默认配置。
//
// 把全表刷成非默认再置目标行，故传 status=N 会导致一个默认都没有，
// 此后上传都会失败——这是前端只在"设为默认"时调用本接口的既有约定。
func (s *OssConfigService) UpdateConfigStatus(ctx context.Context,
	b *bo.SysOssConfigStatusBo) error {

	if b == nil {
		return errors.New("service: 对象存储配置状态入参为空")
	}

	repo := repository.NewOssConfigRepository(database.DB())
	old, err := repo.SelectByID(ctx, b.OssConfigID)
	if err != nil {
		if errors.Is(err, repository.ErrOssConfigNotFound) {
			return ErrOssConfigNotFound
		}
		return err
	}

	if err := repo.UpdateStatus(ctx, b.OssConfigID, b.Status); err != nil {
		return err
	}

	// 全表的 status 都动了，逐条回写缓存，否则 status 字段会与库不一致。
	if err := s.InitCache(ctx); err != nil {
		log.Printf("[oss] 切换默认配置后刷新缓存失败: %v", err)
	}
	if b.Status == constant.Yes {
		s.setDefaultConfigKey(ctx, old.ConfigKey)
	}
	return nil
}
