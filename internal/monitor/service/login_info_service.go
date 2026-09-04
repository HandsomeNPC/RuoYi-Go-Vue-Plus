// Package service 登录日志监控业务逻辑。
package service

import (
	"context"
	"errors"
	"fmt"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	systemrepo "ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/database"
	pkgredis "ruoyi-go-vue-plus/pkg/redis"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

var ErrLoginInfoNotFound = errors.New("service: 登录日志不存在")

// LoginInfoSvc 登录日志监控服务，只做监控侧的查询/删除
// （记录落库在 internal/system/service 的 LoginInfoSvcApp）。
type LoginInfoSvc struct{}

var LoginInfoSvcApp = new(LoginInfoSvc)

// QueryPageList 按条件分页查登录日志。
func (s *LoginInfoSvc) QueryPageList(ctx context.Context, q bo.SysLoginInfoQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*vo.SysLoginInfoVo], error) {

	res, err := systemrepo.NewLoginInfoRepository(database.DB()).SelectPageList(ctx, q, page)
	if err != nil {
		return pkgrepo.EmptyPage[*vo.SysLoginInfoVo](), err
	}
	// 两个 PageResult 泛型实例是不同类型，只能重建；Page 内的空切片兜底顺带保证序列化成 []。
	return pkgrepo.Page(vo.Conv.ConvertToSysLoginInfoVoList(res.Rows), res.Total), nil
}

// QueryList 按条件不分页查登录日志，供导出等全量场景用。
// limit <= 0 不限制行数；导出方应传 excel.MaxRows+1 以提前判定超限，见 pkg/excel 的说明。
func (s *LoginInfoSvc) QueryList(ctx context.Context, q bo.SysLoginInfoQueryBo,
	limit int) ([]*vo.SysLoginInfoVo, error) {

	rows, err := systemrepo.NewLoginInfoRepository(database.DB()).SelectList(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysLoginInfoVoList(rows), nil
}

// DeleteByIDs 批量删除登录日志。未命中的主键静默跳过（不做存在性预校验）。
func (s *LoginInfoSvc) DeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return fmt.Errorf("service: 登录日志主键不能为空")
	}
	if _, err := systemrepo.NewLoginInfoRepository(database.DB()).DeleteByIDs(ctx, ids); err != nil {
		return err
	}
	return nil
}

// Clean 清空全部登录日志。
func (s *LoginInfoSvc) Clean(ctx context.Context) error {
	return systemrepo.NewLoginInfoRepository(database.DB()).Clean(ctx)
}

// Unlock 清除指定用户的登录失败锁定状态。
//
// 直接 Del 而非先 hasKey 再 delete：删除不存在的 key 是 no-op，
// 二者可观察行为一致，少一次往返。key 前缀 pwd_err_cnt: 与 auth 进程写入侧约定相同。
func (s *LoginInfoSvc) Unlock(ctx context.Context, userName string) error {
	if userName == "" {
		return fmt.Errorf("service: 用户名不能为空")
	}
	key := constant.PwdErrCntKeyPrefix + userName
	if err := pkgredis.Client().Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("service: 解锁用户 %s 失败: %w", userName, err)
	}
	return nil
}
