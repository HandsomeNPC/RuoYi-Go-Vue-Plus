package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/model/bo"
	pkgrepo "ruoyi-go-vue-plus/pkg/repository"
)

// ErrClientNotFound 客户端不存在。
var ErrClientNotFound = errors.New("repository: 客户端不存在")

// ClientRepository sys_client 数据访问。
type ClientRepository struct {
	db *gorm.DB
}

// NewClientRepository 构造客户端 repository。
func NewClientRepository(db *gorm.DB) *ClientRepository {
	return &ClientRepository{db: db}
}

// SelectByClientID 按客户端标识查客户端，不存在时返回 ErrClientNotFound。
func (r *ClientRepository) SelectByClientID(ctx context.Context, clientID string) (*model.SysClient, error) {
	if clientID == "" {
		return nil, ErrClientNotFound
	}

	var client model.SysClient
	err := r.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		First(&client).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, fmt.Errorf("repository: 查询客户端 %q 失败: %w", clientID, err)
	}
	return &client, nil
}

// SelectByID 按主键查客户端，不存在时返回 ErrClientNotFound。
func (r *ClientRepository) SelectByID(ctx context.Context, id int64) (*model.SysClient, error) {
	if id <= 0 {
		return nil, ErrClientNotFound
	}

	var client model.SysClient
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&client).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, fmt.Errorf("repository: 查询客户端 id=%d 失败: %w", id, err)
	}
	return &client, nil
}

// SelectPageList 按条件分页查客户端。条件均为空串不筛（对齐 Java eqIfText 语义）。
//
// 返回实体而非 VO：SysClientVo 的三个 *List 字段无对应列，GORM 会按字段名拼进 SELECT。
func (r *ClientRepository) SelectPageList(ctx context.Context, q bo.SysClientQueryBo,
	page pkgrepo.PageQuery) (pkgrepo.PageResult[*model.SysClient], error) {

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	db := r.db.WithContext(ctx).Model(&model.SysClient{})
	if q.ClientID != "" {
		db = db.Where("client_id = ?", q.ClientID)
	}
	if q.ClientKey != "" {
		db = db.Where("client_key = ?", q.ClientKey)
	}
	if q.ClientSecret != "" {
		db = db.Where("client_secret = ?", q.ClientSecret)
	}
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	// 仅在调用方未指定排序时按主键升序兜底，保证翻页稳定。
	// 不能无条件追加：GORM 合并排序子句时先注册的排在前，id 唯一会让后来的排序列失效。
	if !page.HasOrder() {
		db = db.Order("id")
	}

	var rows []*model.SysClient
	return pkgrepo.SelectPage(db, page, &rows)
}

// Insert 插入一条客户端。
// id 无 auto_increment，主键须由调用方（service 层）预先填好；
// 审计字段由 pkg/repository 的插入回调统一填充。
func (r *ClientRepository) Insert(ctx context.Context, client *model.SysClient) error {
	if client == nil {
		return fmt.Errorf("repository: 客户端为空")
	}
	if err := r.db.WithContext(ctx).Create(client).Error; err != nil {
		return fmt.Errorf("repository: 插入客户端失败: %w", err)
	}
	return nil
}

// UpdateByID 按主键更新指定列，返回受影响行数（0 表示主键不存在或已被逻辑删除）。
//
// 走 map 而非实体：Updates(struct) 跳过零值，会让「清空访问路径/IP 白名单」写不进库。
// update_by/update_time 由 pkg/repository 的更新回调补齐，调用方不必带。
func (r *ClientRepository) UpdateByID(ctx context.Context, id int64,
	columns map[string]any) (int64, error) {

	if id <= 0 {
		return 0, fmt.Errorf("repository: 客户端主键无效: %d", id)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("repository: 客户端更新列为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysClient{}).
		Where("id = ?", id).
		Updates(columns)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新客户端 id=%d 失败: %w", id, res.Error)
	}
	return res.RowsAffected, nil
}

// UpdateStatusByClientID 按客户端标识改状态，返回受影响行数（对齐 Java updateClientStatus 的按 client_id 定位）。
func (r *ClientRepository) UpdateStatusByClientID(ctx context.Context, clientID,
	status string) (int64, error) {

	if clientID == "" {
		return 0, fmt.Errorf("repository: 客户端标识为空")
	}

	res := r.db.WithContext(ctx).
		Model(&model.SysClient{}).
		Where("client_id = ?", clientID).
		Update("status", status)
	if res.Error != nil {
		return 0, fmt.Errorf("repository: 更新客户端 %q 状态失败: %w", clientID, res.Error)
	}
	return res.RowsAffected, nil
}

// ExistsByClientKey 判断 client_key 是否已被占用，excludeID > 0 时排除该主键
// （对齐 Java neIfPresent，供修改场景排除自身）。
func (r *ClientRepository) ExistsByClientKey(ctx context.Context, clientKey string,
	excludeID int64) (bool, error) {

	if clientKey == "" {
		return false, nil
	}

	// Model 不能省：del_flag 过滤由 LogicDelete 挂在字段类型上，须先解析出实体 schema 才生效。
	db := r.db.WithContext(ctx).Model(&model.SysClient{}).Where("client_key = ?", clientKey)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return false, fmt.Errorf("repository: 校验客户端 key %q 失败: %w", clientKey, err)
	}
	return count > 0, nil
}
