package service

import (
	"context"

	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
)

// PostService 岗位业务逻辑。
type PostService struct{}

// PostSvcApp 包级实例。
var PostSvcApp = new(PostService)

// SelectPostsByUserId 按用户ID查岗位列表（对应 Java SysPostServiceImpl#selectPostsByUserId）。
func (s *PostService) SelectPostsByUserId(ctx context.Context, userID int64) ([]*vo.SysPostVo, error) {
	posts, err := repository.NewPostRepository(database.DB()).SelectPostsByUserId(ctx, userID)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysPostVoList(posts), nil
}
