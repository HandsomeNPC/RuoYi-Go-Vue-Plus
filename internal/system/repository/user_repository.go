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
var ErrUserNotFound = errors.New("repository: 用户不存在")

// UserRepository sys_user 数据访问。
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository 构造用户 repository。
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// SelectByUserID 按用户ID查用户，不存在时返回 ErrUserNotFound。
func (r *UserRepository) SelectByUserID(ctx context.Context, userID int64) (*model.SysUser, error) {
	if userID == 0 {
		return nil, ErrUserNotFound
	}

	var user model.SysUser
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("repository: 查询用户 %d 失败: %w", userID, err)
	}
	return &user, nil
}

// SelectByUserName 按用户账号查用户，不存在时返回 ErrUserNotFound。
func (r *UserRepository) SelectByUserName(ctx context.Context, userName string) (*model.SysUser, error) {
	if userName == "" {
		return nil, ErrUserNotFound
	}

	var user model.SysUser
	err := r.db.WithContext(ctx).
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

// SelectUserNamesByIDs 按用户ID批量取账号，返回 id → user_name 映射
// （对应 Java selectUserNameById 的批量形态，供 USER_ID_TO_NAME 翻译用）。
//
// 一次 IN 查询而非逐个查：列表页每行都要翻译创建人，单查会打出 N+1。
// 缺失的 ID 不出现在结果里，由调用方按空串兜底。
func (r *UserRepository) SelectUserNamesByIDs(ctx context.Context,
	userIDs []int64) (map[int64]string, error) {

	if len(userIDs) == 0 {
		return map[int64]string{}, nil
	}

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	var rows []struct {
		UserID   int64
		UserName string
	}
	err := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Select("user_id", "user_name").
		Where("user_id IN ?", userIDs).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("repository: 批量查询用户账号 %v 失败: %w", userIDs, err)
	}

	names := make(map[int64]string, len(rows))
	for _, row := range rows {
		names[row.UserID] = row.UserName
	}
	return names, nil
}

// ExistsByDeptID 判断部门下是否已分配用户（对应 Java checkDeptExistUser）。
func (r *UserRepository) ExistsByDeptID(ctx context.Context, deptID int64) (bool, error) {
	if deptID <= 0 {
		return false, nil
	}

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	var count int64
	err := r.db.WithContext(ctx).Model(&model.SysUser{}).
		Where("dept_id = ?", deptID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("repository: 查询部门 %d 下的用户失败: %w", deptID, err)
	}
	return count > 0, nil
}

// UpdateLoginInfo 更新最后登录 IP 与时间。
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

// ExistsByPhone 判断手机号是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java checkPhoneUnique 的 neIfPresent，供个人中心改资料排除自身）。
func (r *UserRepository) ExistsByPhone(ctx context.Context, phone string,
	excludeID int64) (bool, error) {

	if phone == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysUser{}).Where("phone_number = ?", phone)
	if excludeID > 0 {
		db = db.Where("user_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验手机号 %q 失败: %w", phone, err)
	}
	return count > 0, nil
}

// ExistsByEmail 判断邮箱是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java checkEmailUnique 的 neIfPresent，供个人中心改资料排除自身）。
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string,
	excludeID int64) (bool, error) {

	if email == "" {
		return false, nil
	}

	db := r.db.WithContext(ctx).Model(&model.SysUser{}).Where("email = ?", email)
	if excludeID > 0 {
		db = db.Where("user_id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验邮箱 %q 失败: %w", email, err)
	}
	return count > 0, nil
}

// UpdateProfile 按主键更新个人资料（对应 Java updateUserProfile）。
//
// 走 map 且仅写非空字段——对齐 Java setIfPresent：个人资料表单字段全可选，
// 漏传的字段不该把线上的昵称/邮箱刷成空串。update_by/update_time 由回调补齐。
func (r *UserRepository) UpdateProfile(ctx context.Context, userID int64,
	columns map[string]any) (int64, error) {

	if userID == 0 {
		return 0, errors.New("repository: userID 不能为 0")
	}
	if len(columns) == 0 {
		return 0, errors.New("repository: 个人资料更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新用户 %d 个人资料失败: %w", userID, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdatePassword 按主键重置密码（对应 Java resetUserPwd）。
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int64,
	password string) (int64, error) {

	if userID == 0 {
		return 0, errors.New("repository: userID 不能为 0")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysUser{}).
		Where("user_id = ?", userID).
		Update("password", password)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 重置用户 %d 密码失败: %w", userID, res.Error)
	}
	return res.RowsAffected, nil
}
