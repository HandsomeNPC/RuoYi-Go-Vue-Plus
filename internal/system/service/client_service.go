package service

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"ruoyi-go-vue-plus/internal/system/model"
	"ruoyi-go-vue-plus/internal/system/repository"
)

// ErrClientNotFound 客户端不存在，转自 repository 层（理由同 ErrUserNotFound）。
var ErrClientNotFound = errors.New("service: 客户端不存在")

// ClientService 客户端业务逻辑。
//
// 导出给 internal/auth 复用：登录时要按 clientId 查出授权类型、
// 设备类型、超时时长与访问规则。
type ClientService struct {
	repo *repository.ClientRepository
}

// NewClientService 构造客户端 service。
func NewClientService(db *gorm.DB) *ClientService {
	return &ClientService{repo: repository.NewClientRepository(db)}
}

// GetByClientID 按客户端标识查客户端，不存在时返回 ErrClientNotFound。
//
// 对应 Java ISysClientService.queryByClientId。
//
// **状态与授权类型的校验不在这里做**，归调用方（internal/auth/service）——
// 那两条判断各自对应一句特定的登录失败文案
// （"认证权限类型错误" / "认证权限类型已禁用"，见 AuthController.java:81-90），
// 属于认证流程而非客户端查询本身。本方法将来也要服务于客户端管理接口
// （阶段 3），那里查停用的客户端是正常需求。
func (s *ClientService) GetByClientID(ctx context.Context, clientID string) (*model.SysClient, error) {
	client, err := s.repo.SelectByClientID(ctx, clientID)
	if err != nil {
		if errors.Is(err, repository.ErrClientNotFound) {
			return nil, ErrClientNotFound
		}
		return nil, err
	}
	return client, nil
}
