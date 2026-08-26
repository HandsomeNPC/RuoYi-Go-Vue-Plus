package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
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
