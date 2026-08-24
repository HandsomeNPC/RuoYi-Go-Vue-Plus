// Package repository system 模块数据访问层(GORM)。
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
)

// ErrUserNotFound 用户不存在。
//
// 包装成本层的哨兵错误而非直接漏 gorm.ErrRecordNotFound：
// service 层不该为了判断「有没有这个用户」而 import gorm ——
// 那是数据访问细节，换掉 ORM 时不该波及上层。
var ErrUserNotFound = errors.New("repository: 用户不存在")

// UserRepository sys_user 数据访问。
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造用户 repository。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// SelectByUserName 按用户账号查用户，不存在时返回 ErrUserNotFound。
//
// 对应 Java PasswordAuthStrategy.loadUserByUsername 里的
// userMapper.lambda().eq(SysUser::getUserName, username).voOne()。
//
// 查出来的 SysUser **带 Password 字段**（BCrypt 哈希），登录要用它比对。
// 该字段有 json:"-"，不会漏进响应体。
func (r *UserRepository) SelectByUserName(ctx context.Context, userName string) (*model.SysUser, error) {
	if userName == "" {
		return nil, ErrUserNotFound
	}

	var user model.SysUser
	err := r.db.WithContext(ctx).
		Scopes(NotDeleted()).
		Where("user_name = ?", userName).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("repository: 查询用户 %q 失败: %w", userName, err)
	}
	return &user, nil
}

// UpdateLoginInfo 更新最后登录 IP 与时间。
//
// 对应 Java SysLoginService.updateLastLoginInfo（:190-197）。
//
// **必须用 Updates 指定列而非 Save 整个实体**：Save 会写回所有字段，
// 包括 Password —— 而调用方传进来的实体若是从别处构造的（没带哈希），
// 就会把密码清空。Java 侧靠 @TableField(updateStrategy = NOT_EMPTY)
// 让空密码不进 update 语句来避免这件事，Go 侧靠只更新指定列。
//
// update_by 也一并写成本人，对齐 Java 的 sysUser.setUpdateBy(userId)。
func (r *UserRepository) UpdateLoginInfo(ctx context.Context, userID int64, ip string) error {
	if userID == 0 {
		return errors.New("repository: userID 不能为 0")
	}

	now := time.Now()
	err := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{
			"login_ip":    ip,
			"login_date":  now,
			"update_by":   userID,
			"update_time": now,
		}).Error
	if err != nil {
		return fmt.Errorf("repository: 更新用户 %d 登录信息失败: %w", userID, err)
	}
	return nil
}
