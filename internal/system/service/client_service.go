package service

import (
	"context"
	"errors"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// ErrClientNotFound 客户端不存在。
var ErrClientNotFound = errors.New("service: 客户端不存在")

// ClientService 客户端业务逻辑。
type ClientService struct{}

// ClientSvcApp 包级实例。
var ClientSvcApp = new(ClientService)

// GetByClientID 按客户端标识查客户端，不存在时返回 ErrClientNotFound。
func (s *ClientService) GetByClientID(ctx context.Context, clientID string) (*model.SysClient, error) {
	client, err := repository.NewClientRepository(database.DB()).SelectByClientID(ctx, clientID)
	if err != nil {
		if errors.Is(err, repository.ErrClientNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	return client, nil
}
