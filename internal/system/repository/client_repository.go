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
//
// 对应 Java SysClientServiceImpl.queryByClientId（:60-66）。
//
// **有意不做缓存**。Java 侧挂了 @Cacheable(cacheNames = "sys_client#30d")，
// 30 天 TTL。Go 侧当前不加，两个理由：
//
//   - 这个查询只在**登录时**发生一次（鉴权路径读的是 JWT claims 里冻结的
//     规则，不查库，见 pkg/middleware/auth.go 的 checkClientRules）。
//     登录本身要做 BCrypt 比对（cost 10，约 50-100ms），
//     一次主键索引查询在其中占比可忽略。
//   - 缓存要配套失效逻辑（Java 侧在 update/status/delete 三处 @CacheEvict）。
//     阶段 3 做客户端管理接口时才有那些写入口，届时一并加缓存与失效
//     才能保证两者不脱节 —— 现在只加缓存，等于埋一个「改了客户端配置
//     30 天不生效」的坑。
//
// pkg/constant 里已备好 CacheSysClient 与 CacheTTLSysClient 供那时使用。
func (r *ClientRepository) SelectByClientID(ctx context.Context, clientID string) (*model.SysClient, error) {
	if clientID == "" {
		return nil, ErrClientNotFound
	}

	var client model.SysClient
	err := r.db.WithContext(ctx).
		Scopes(NotDeleted()).
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
