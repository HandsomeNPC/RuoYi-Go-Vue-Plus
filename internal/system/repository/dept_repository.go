package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// ErrDeptNotFound 部门不存在。
var ErrDeptNotFound = errors.New("repository: 部门不存在")

// DeptRepository sys_dept 数据访问。
type DeptRepository struct {
	db *gorm.DB
}

// NewDeptRepository 构造部门 repository。
func NewDeptRepository(db *gorm.DB) *DeptRepository {
	return &DeptRepository{db: db}
}

// SelectByID 按主键查部门（对应 Java SysDeptMapper#selectVoById），不存在时返回 ErrDeptNotFound。
// 仅取实体本身；父部门名等扩展字段由 service 层按需回填，buildLoginUser 只读 DeptName/DeptCategory。
func (r *DeptRepository) SelectByID(ctx context.Context, deptID int64) (*model.SysDept, error) {
	if deptID <= 0 {
		return nil, ErrDeptNotFound
	}

	var dept model.SysDept
	err := r.db.WithContext(ctx).
		Where("dept_id = ?", deptID).
		First(&dept).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDeptNotFound
		}
		return nil, fmt.Errorf("repository: 查询部门 id=%d 失败: %w", deptID, err)
	}
	return &dept, nil
}
