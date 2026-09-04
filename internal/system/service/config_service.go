package service

import (
	"context"
	"errors"
	"fmt"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/cache"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/errs"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

var ErrConfigNotFound = errors.New("service: 参数配置不存在")

var ErrConfigKeyExists = errors.New("service: 参数键名已存在")

// ConfigService 参数配置业务逻辑。
type ConfigService struct{}

var ConfigSvcApp = new(ConfigService)

// QueryPageList 按条件分页查参数配置。
func (s *ConfigService) QueryPageList(ctx context.Context, q bo.SysConfigQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysConfigVo], error) {

	res, err := repository.NewConfigRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysConfigVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(vo.Conv.ConvertToSysConfigVoList(res.Rows), res.Total), nil
}

// QueryList 按条件不分页查参数配置，供导出等全量场景用。
// limit <= 0 不限制行数；导出方应传 excel.MaxRows+1 以提前判定超限，见 pkg/excel 的说明。
func (s *ConfigService) QueryList(ctx context.Context, q bo.SysConfigQueryBo,
	limit int) ([]*vo.SysConfigVo, error) {

	rows, err := repository.NewConfigRepository(database.DB()).SelectList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysConfigVoList(rows), nil
}

// QueryByID 按主键查参数配置，不存在时返回 ErrConfigNotFound。
func (s *ConfigService) QueryByID(ctx context.Context, configID int64) (*vo.SysConfigVo, error) {
	cfg, err := repository.NewConfigRepository(database.DB()).SelectByID(ctx, configID)
	if err != nil {
		if errors.Is(err, repository.ErrConfigNotFound) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysConfigVo(cfg), nil
}

// SelectConfigByKey 按参数键名取参数值。
//
// 键不存在时返回空串而非报错，且这个空串同样入缓存——缓存的是返回值，
// 不区分「查到空」与「没查到」。照搬这一点是必要的：insertConfig 会在新增时
// 把该键刷成真值，不会留下永久的空缓存。
func (s *ConfigService) SelectConfigByKey(ctx context.Context, configKey string) (string, error) {
	var cached string
	if hit, _ := cache.Get(ctx, constant.CacheSysConfig, configKey, &cached); hit {
		return cached, nil
	}

	value := ""
	cfg, err := repository.NewConfigRepository(database.DB()).SelectByKey(ctx, configKey)
	if err != nil && !errors.Is(err, repository.ErrConfigNotFound) {
		return "", err
	}
	if cfg != nil {
		value = cfg.ConfigValue
	}

	_ = cache.Put(ctx, constant.CacheSysConfig, configKey, value, constant.CacheTTLSysConfig)
	return value, nil
}

// CheckConfigKeyUnique 校验 config_key 是否可用（唯一即 true）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *ConfigService) CheckConfigKeyUnique(ctx context.Context, configKey string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewConfigRepository(database.DB()).
		ExistsByConfigKey(ctx, configKey, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// InsertConfig 新增参数配置。
// config_key 重复时返回 ErrConfigKeyExists；插入成功后回填 b.ConfigID。
func (s *ConfigService) InsertConfig(ctx context.Context, b *bo.SysConfigBo) error {
	if b == nil {
		return errors.New("service: 参数配置入参为空")
	}

	unique, err := s.CheckConfigKeyUnique(ctx, b.ConfigKey, 0) // 新增无自身可排除
	if err != nil {
		return err
	}
	if !unique {
		return ErrConfigKeyExists
	}

	add := bo.Conv.ConvertToSysConfig(b)
	add.ConfigID = snowflake.Next() // config_id 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewConfigRepository(database.DB()).Insert(ctx, add); err != nil {
		return err
	}
	b.ConfigID = add.ConfigID
	// 新增即把键值写进缓存，顺带覆盖 SelectConfigByKey 可能缓存过的空串。
	_ = cache.Put(ctx, constant.CacheSysConfig, add.ConfigKey, add.ConfigValue,
		constant.CacheTTLSysConfig)
	return nil
}

// UpdateConfig 按主键修改参数配置。
// config_key 被别的配置占用时返回 ErrConfigKeyExists；主键不存在返回 ErrConfigNotFound。
func (s *ConfigService) UpdateConfig(ctx context.Context, b *bo.SysConfigBo) error {
	if b == nil {
		return errors.New("service: 参数配置入参为空")
	}
	if b.ConfigID <= 0 {
		return errors.New("service: 参数配置主键不能为空")
	}

	unique, err := s.CheckConfigKeyUnique(ctx, b.ConfigKey, b.ConfigID) // 排除自身，改回原 key 不算冲突
	if err != nil {
		return err
	}
	if !unique {
		return ErrConfigKeyExists
	}

	repo := repository.NewConfigRepository(database.DB())

	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行（update_time 是秒精度，同秒内重复提交连它都不变），
	// 那会把一次幂等的重复保存误报成"配置不存在"。
	old, err := repo.SelectByID(ctx, b.ConfigID)
	if err != nil {
		if errors.Is(err, repository.ErrConfigNotFound) {
			return ErrConfigNotFound
		}
		return err
	}
	// 改键名时旧键的缓存会成为无人回收的孤儿，先按旧键失效。
	if old.ConfigKey != b.ConfigKey {
		_ = cache.Evict(ctx, constant.CacheSysConfig, old.ConfigKey)
	}

	if _, err := repo.UpdateByID(ctx, b.ConfigID, buildConfigUpdateColumns(b)); err != nil {
		return err
	}
	_ = cache.Put(ctx, constant.CacheSysConfig, b.ConfigKey, b.ConfigValue,
		constant.CacheTTLSysConfig)
	return nil
}

// buildConfigUpdateColumns 组装修改参数配置的更新列。
func buildConfigUpdateColumns(b *bo.SysConfigBo) map[string]any {
	columns := map[string]any{
		"config_name":  b.ConfigName,
		"config_key":   b.ConfigKey,
		"config_value": b.ConfigValue,
		// 一律写入，让前端能把备注清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		"remark": b.Remark,
	}
	// 内置标记缺省即视为不改：漏传字段不该把线上的 'Y' 刷成空串，
	// 那会让内置参数失去删除保护。
	if b.ConfigType != "" {
		columns["config_type"] = b.ConfigType
	}
	return columns
}

// UpdateConfigByKey 按参数键名修改参数值。
// 键名不存在时返回 ErrConfigNotFound。
//
// 只改 config_value：入参仅这两个字段，把 name/type/remark 一并写空会抹掉现有配置。
func (s *ConfigService) UpdateConfigByKey(ctx context.Context, b *bo.SysConfigUpdateByKeyBo) error {
	if b == nil {
		return errors.New("service: 参数配置入参为空")
	}
	if b.ConfigKey == "" {
		return errors.New("service: 参数键名不能为空")
	}

	repo := repository.NewConfigRepository(database.DB())
	// 先定存在性，同上：值未变时 UpdateByKey 会报 0 行，不能据此判定键名不存在。
	if _, err := repo.SelectByKey(ctx, b.ConfigKey); err != nil {
		if errors.Is(err, repository.ErrConfigNotFound) {
			return ErrConfigNotFound
		}
		return err
	}

	// 先失效再更新：
	// 更新失败时缓存宁可空着走一次 DB，也不能留着与库不一致的旧值。
	_ = cache.Evict(ctx, constant.CacheSysConfig, b.ConfigKey)

	// 在此处收成 string，不把 LooseString 带进 repository 与缓存：
	// 那个宽容类型只为兼容入参形态而存在，落库与缓存都该是普通字符串。
	configValue := b.ConfigValue.String()
	if _, err := repo.UpdateByKey(ctx, b.ConfigKey,
		map[string]any{"config_value": configValue}); err != nil {
		return err
	}
	_ = cache.Put(ctx, constant.CacheSysConfig, b.ConfigKey, configValue,
		constant.CacheTTLSysConfig)
	return nil
}

// DeleteConfigByIDs 批量删除参数配置。
// 命中任一内置参数（config_type = 'Y'）即整批拒绝，不做部分删除。
func (s *ConfigService) DeleteConfigByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("service: 参数配置主键不能为空")
	}

	repo := repository.NewConfigRepository(database.DB())
	rows, err := repo.SelectByIDs(ctx, ids)
	if err != nil {
		return err
	}

	// 先整批校验再删，不边删边校验：这里没有事务包裹，
	// 一旦先删了几行再撞上内置参数就会留下删一半的状态。
	for _, cfg := range rows {
		if cfg.ConfigType == constant.Yes {
			return errs.New(0, fmt.Sprintf("内置参数【%s】不能删除", cfg.ConfigKey), "")
		}
	}

	if _, err := repo.DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	// 删除后再失效：提前清缓存会让删除失败时白丢一批热数据。
	// 按实际命中的行逐个清，缺失的主键本来也没有对应缓存。
	for _, cfg := range rows {
		_ = cache.Evict(ctx, constant.CacheSysConfig, cfg.ConfigKey)
	}
	return nil
}

// ResetConfigCache 清空参数缓存。
func (s *ConfigService) ResetConfigCache(ctx context.Context) error {
	return cache.EvictGroup(ctx, constant.CacheSysConfig)
}
