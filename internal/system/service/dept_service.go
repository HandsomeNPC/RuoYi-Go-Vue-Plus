package service

import (
	"context"
	"errors"

	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// DeptService 部门业务逻辑。
type DeptService struct{}

// DeptSvcApp 包级实例。
var DeptSvcApp = new(DeptService)

// SelectByID 按主键查部门（对应 Java SysDeptServiceImpl#selectDeptById），
// 不存在时返回 (nil, nil)，由调用方按空串兜底（对应 Java Opt.orElse(StringUtils.EMPTY)）。
func (s *DeptService) SelectByID(ctx context.Context, deptID int64) (*vo.SysDeptVo, error) {
	dept, err := repository.NewDeptRepository(database.DB()).SelectByID(ctx, deptID)
	if err != nil {
		if errors.Is(err, repository.ErrDeptNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return vo.Conv.ConvertToSysDeptVo(dept), nil
}
