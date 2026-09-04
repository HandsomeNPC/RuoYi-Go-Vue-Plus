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

var ErrDictTypeNotFound = errors.New("service: 字典类型不存在")

var ErrDictTypeExists = errors.New("service: 字典类型已存在")

// DictTypeService 字典类型业务逻辑。
type DictTypeService struct{}

var DictTypeSvcApp = new(DictTypeService)

// QueryPageList 按条件分页查字典类型。
func (s *DictTypeService) QueryPageList(ctx context.Context, q bo.SysDictTypeQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysDictTypeVo], error) {

	res, err := repository.NewDictTypeRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysDictTypeVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(vo.Conv.ConvertToSysDictTypeVoList(res.Rows), res.Total), nil
}

// QueryList 按条件不分页查字典类型，供导出等全量场景用。
// limit <= 0 不限制行数；导出方应传 excel.MaxRows+1 以提前判定超限，见 pkg/excel 的说明。
func (s *DictTypeService) QueryList(ctx context.Context, q bo.SysDictTypeQueryBo,
	limit int) ([]*vo.SysDictTypeVo, error) {

	rows, err := repository.NewDictTypeRepository(database.DB()).SelectList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysDictTypeVoList(rows), nil
}

// QueryAll 查全部字典类型，供下拉选择用。
func (s *DictTypeService) QueryAll(ctx context.Context) ([]*vo.SysDictTypeVo, error) {
	return s.QueryList(ctx, bo.SysDictTypeQueryBo{}, 0)
}

// QueryByID 按主键查字典类型，不存在时返回 ErrDictTypeNotFound。
func (s *DictTypeService) QueryByID(ctx context.Context, dictID int64) (*vo.SysDictTypeVo, error) {
	dict, err := repository.NewDictTypeRepository(database.DB()).SelectByID(ctx, dictID)
	if err != nil {
		if errors.Is(err, repository.ErrDictTypeNotFound) {
			return nil, ErrDictTypeNotFound
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysDictTypeVo(dict), nil
}

// SelectDictDataByType 按字典类型查字典数据。
//
// 类型不存在时返回空切片而非报错，且这个空切片同样入缓存——意在
// 防缓存穿透（新建类型下还没有数据时同样不该反复打库）。
func (s *DictTypeService) SelectDictDataByType(ctx context.Context,
	dictType string) ([]*vo.SysDictDataVo, error) {

	return selectDictDataByTypeCached(ctx, dictType)
}

// CheckDictTypeUnique 校验 dict_type 是否可用（唯一即 true）。
// excludeID > 0 时排除该主键，供修改场景复用。
func (s *DictTypeService) CheckDictTypeUnique(ctx context.Context, dictType string,
	excludeID int64) (bool, error) {

	exists, err := repository.NewDictTypeRepository(database.DB()).
		ExistsByDictType(ctx, dictType, excludeID)
	if err != nil {
		return false, err
	}
	return !exists, nil
}

// InsertDictType 新增字典类型。
// dict_type 重复时返回 ErrDictTypeExists；插入成功后回填 b.DictID。
func (s *DictTypeService) InsertDictType(ctx context.Context, b *bo.SysDictTypeBo) error {
	if b == nil {
		return errors.New("service: 字典类型入参为空")
	}
	if err := validateDictTypeFormat(b.DictType); err != nil {
		return err
	}

	unique, err := s.CheckDictTypeUnique(ctx, b.DictType, 0) // 新增无自身可排除
	if err != nil {
		return err
	}
	if !unique {
		return ErrDictTypeExists
	}

	add := bo.Conv.ConvertToSysDictType(b)
	add.DictID = snowflake.Next() // dict_id 无 auto_increment
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。

	if err := repository.NewDictTypeRepository(database.DB()).Insert(ctx, add); err != nil {
		return err
	}
	b.DictID = add.DictID
	// 新类型下必然还没有字典数据，写入空列表防缓存穿透。
	_ = cache.Put(ctx, constant.CacheSysDict, add.DictType, []*vo.SysDictDataVo{},
		constant.CacheTTLSysDict)
	return nil
}

// UpdateDictType 修改字典类型。
//
// 改类型名会联动改写 sys_dict_data.dict_type：两表靠这个字符串关联而非外键，
// 不联动会让该类型下的字典数据全部失联。
func (s *DictTypeService) UpdateDictType(ctx context.Context, b *bo.SysDictTypeBo) error {
	if b == nil {
		return errors.New("service: 字典类型入参为空")
	}
	if b.DictID <= 0 {
		return errors.New("service: 字典类型主键不能为空")
	}
	if err := validateDictTypeFormat(b.DictType); err != nil {
		return err
	}

	unique, err := s.CheckDictTypeUnique(ctx, b.DictType, b.DictID) // 排除自身，改回原值不算冲突
	if err != nil {
		return err
	}
	if !unique {
		return ErrDictTypeExists
	}

	repo := repository.NewDictTypeRepository(database.DB())
	// 先取原记录定存在性，不靠 UpdateByID 的受影响行数：值与库中完全相同时
	// MySQL 报 0 行（update_time 是秒精度，同秒内重复提交连它都不变），
	// 那会把一次幂等的重复保存误报成"字典类型不存在"。
	old, err := repo.SelectByID(ctx, b.DictID)
	if err != nil {
		if errors.Is(err, repository.ErrDictTypeNotFound) {
			return ErrDictTypeNotFound
		}
		return err
	}

	// 先联动字典数据再改类型本身：这里没有事务包裹，反序一旦中途失败，
	// 字典数据会挂在一个已不存在的类型名下且无从追溯原值。
	if old.DictType != b.DictType {
		if _, err := repository.NewDictDataRepository(database.DB()).
			UpdateTypeByType(ctx, old.DictType, b.DictType); err != nil {
			return err
		}
	}

	if _, err := repo.UpdateByID(ctx, b.DictID, buildDictTypeUpdateColumns(b)); err != nil {
		return err
	}

	// 旧类型名的两组缓存会成为无人回收的孤儿，按旧名失效。
	if old.DictType != b.DictType {
		_ = cache.Evict(ctx, constant.CacheSysDict, old.DictType)
		_ = cache.Evict(ctx, constant.CacheSysDictType, old.DictType)
	}
	// 回写新类型名下的字典数据列表。查库失败不影响改类型本身已成功，
	// 缓存留空下次读时自然回填。
	if list, err := selectDictDataByTypeUncached(ctx, b.DictType); err == nil {
		_ = cache.Put(ctx, constant.CacheSysDict, b.DictType, list, constant.CacheTTLSysDict)
	}
	return nil
}

// buildDictTypeUpdateColumns 组装修改字典类型的更新列。
func buildDictTypeUpdateColumns(b *bo.SysDictTypeBo) map[string]any {
	return map[string]any{
		"dict_name": b.DictName,
		"dict_type": b.DictType,
		// 一律写入，让前端能把备注清空——这正是编辑表单的本意，
		// 故不能用 Updates(struct)（它跳过零值，空串会被当成"未修改"而丢弃）。
		"remark": b.Remark,
	}
}

// DeleteDictTypeByIDs 批量删除字典类型。
// 任一类型下已有字典数据即整批拒绝，不做部分删除。
func (s *DictTypeService) DeleteDictTypeByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return errors.New("service: 字典类型主键不能为空")
	}

	repo := repository.NewDictTypeRepository(database.DB())
	rows, err := repo.SelectByIDs(ctx, ids)
	if err != nil {
		return err
	}

	// 先整批校验再删，不边删边校验：这里没有事务包裹，
	// 一旦先删了几行再撞上已分配的类型就会留下删一半的状态。
	dataRepo := repository.NewDictDataRepository(database.DB())
	for _, dict := range rows {
		assigned, err := dataRepo.ExistsByType(ctx, dict.DictType)
		if err != nil {
			return err
		}
		if assigned {
			return errs.New(0, fmt.Sprintf("%s已分配,不能删除", dict.DictName), "")
		}
	}

	if _, err := repo.DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	// 删除后再失效：提前清缓存会让删除失败时白丢一批热数据。
	// 按实际命中的行逐个清，缺失的主键本来也没有对应缓存。
	for _, dict := range rows {
		_ = cache.Evict(ctx, constant.CacheSysDict, dict.DictType)
		_ = cache.Evict(ctx, constant.CacheSysDictType, dict.DictType)
	}
	return nil
}

// ResetDictCache 清空字典缓存。
// 两组一起清：SYS_DICT 存类型下的数据列表，SYS_DICT_TYPE 存类型本身，只清一组会留下不一致。
func (s *DictTypeService) ResetDictCache(ctx context.Context) error {
	if err := cache.EvictGroup(ctx, constant.CacheSysDict); err != nil {
		return err
	}
	return cache.EvictGroup(ctx, constant.CacheSysDictType)
}

// validateDictTypeFormat 校验字典类型命名格式。
//
// 放在 service 而非 BO 的 binding tag：gin 默认的 validator 没有注册这条正则，
// 而字典类型是 sys_dict_data 的关联键，格式失守会让前端按类型取字典时静默取空。
func validateDictTypeFormat(dictType string) error {
	if !constant.PatternDictionaryType.MatchString(dictType) {
		return errs.New(0, "字典类型必须以字母开头，且只能为（小写字母，数字，下滑线）", dictType)
	}
	return nil
}

// selectDictDataByTypeCached 读穿 SYS_DICT 缓存取某类型下的字典数据。
//
// 供字典类型与字典数据两个 service 共用：读方法挂在这里，
// 而字典数据的写方法要靠它回填缓存，两边指的是同一份缓存。
func selectDictDataByTypeCached(ctx context.Context, dictType string) ([]*vo.SysDictDataVo, error) {
	var cached []*vo.SysDictDataVo
	if hit, _ := cache.Get(ctx, constant.CacheSysDict, dictType, &cached); hit {
		return cached, nil
	}

	list, err := selectDictDataByTypeUncached(ctx, dictType)
	if err != nil {
		return nil, err
	}
	_ = cache.Put(ctx, constant.CacheSysDict, dictType, list, constant.CacheTTLSysDict)
	return list, nil
}

// selectDictDataByTypeUncached 直接查库取某类型下的字典数据，恒返回非 nil 切片
// （同时保证 JSON 序列化成 [] 而非 null）。
func selectDictDataByTypeUncached(ctx context.Context, dictType string) ([]*vo.SysDictDataVo, error) {
	rows, err := repository.NewDictDataRepository(database.DB()).SelectByType(ctx, dictType)
	if err != nil {
		return nil, err
	}
	list := vo.Conv.ConvertToSysDictDataVoList(rows)
	if list == nil {
		return []*vo.SysDictDataVo{}, nil
	}
	return list, nil
}
