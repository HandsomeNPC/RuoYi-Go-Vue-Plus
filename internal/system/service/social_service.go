package service

import (
	"context"

	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// SocialService 社会化关系业务逻辑（对应 Java SysSocialServiceImpl）。
type SocialService struct{}

// SocialSvcApp 包级实例。
var SocialSvcApp = new(SocialService)

// QueryListByUserId 按用户ID查其绑定的社会化授权列表（对应 Java queryListByUserId）。
// 当前用户查不到属空集，不算错。
func (s *SocialService) QueryListByUserId(ctx context.Context,
	userID int64) ([]*vo.SysSocialVo, error) {

	rows, err := repository.NewSocialRepository(database.DB()).SelectByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysSocialVoList(rows), nil
}
