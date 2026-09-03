package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/vo"
	"ruoyi-go-vue-plus/internal/system/repository"
	"ruoyi-go-vue-plus/pkg/database"
	"ruoyi-go-vue-plus/pkg/redis"
	"ruoyi-go-vue-plus/pkg/snowflake"
)

// SocialService 社会化关系业务逻辑（对应 Java SysSocialServiceImpl）。
type SocialService struct{}

// SocialSvcApp 包级实例。
var SocialSvcApp = new(SocialService)

// ErrSocialAlreadyBound 该三方账号已被其他用户绑定。
var ErrSocialAlreadyBound = errors.New("service: 此三方账号已经被绑定")

// ErrSocialNotFound 社会化绑定不存在。
var ErrSocialNotFound = errors.New("service: 社会化绑定不存在")

// socialRegisterLockTTL 绑定操作的互斥锁存活时间。
// 取 10s：够一次「查重 + 写库」往返，又不至于在进程崩溃后长期锁死该 authId。
const socialRegisterLockTTL = 10 * time.Second

// socialRegisterLockKey 绑定互斥锁的键前缀，后接 authId。
const socialRegisterLockKey = "global:lock:social_register:"

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

// SelectByAuthId 按 authId 查绑定关系（对应 Java selectByAuthId）。
func (s *SocialService) SelectByAuthId(ctx context.Context,
	authID string) ([]*vo.SysSocialVo, error) {

	rows, err := repository.NewSocialRepository(database.DB()).SelectByAuthID(ctx, authID)
	if err != nil {
		return nil, err
	}
	return vo.Conv.ConvertToSysSocialVoList(rows), nil
}

// SaveOrUpdate 绑定或续绑一个三方账号，承接 Java SysLoginService.socialRegister 的判定：
// authId 已被他人占用则拒绝；本人已绑过同平台则更新令牌，否则新增。
//
// Java 侧靠 @Lock4j 分布式锁挡住同一 authId 的并发绑定，本项目无该设施，
// 故用 Redis SetNX 自建一把（键按 authId 分桶）。不加锁的话两个并发请求
// 会各自查重通过、双双插入，留下两行同 auth_id 的脏数据。
func (s *SocialService) SaveOrUpdate(ctx context.Context, b *bo.SysSocialBo) error {
	if b == nil || b.AuthID == "" || b.UserID <= 0 {
		return fmt.Errorf("service: 社会化绑定入参不完整")
	}

	unlock, ok, err := acquireSocialLock(ctx, b.AuthID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("service: 该三方账号正在绑定中，请稍后重试")
	}
	defer unlock()

	repo := repository.NewSocialRepository(database.DB())

	// 已被绑定就拒绝——包括被自己绑定：Java 在 authId 命中时无条件抛异常，
	// 同一账号重复点绑定按「已绑定」提示，而非静默续绑。
	bound, err := repo.SelectByAuthID(ctx, b.AuthID)
	if err != nil {
		return err
	}
	if len(bound) > 0 {
		return ErrSocialAlreadyBound
	}

	// 同一用户在同平台已有绑定则更新那条（换了三方账号重新授权的情形），否则新增。
	existing, err := repo.SelectByUserIDAndSource(ctx, b.UserID, b.Source)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		_, err = repo.UpdateByID(ctx, existing[0].ID, buildSocialUpdateColumns(b))
		return err
	}

	add := bo.Conv.ConvertToSysSocial(b)
	// 各业务表主键均无 auto_increment，须自行发号。
	add.ID = snowflake.Next()
	// 审计字段不在此处填：由 pkg/repository 的插入回调统一注入。
	return repo.Insert(ctx, add)
}

// DeleteWithValidById 删除一条社会化绑定，返回是否删到
// （对应 Java deleteWithValidById 的 deleteById > 0）。
//
// 受影响行数为 0 即视为失败，是删除类接口的既定口径（见 docs/CRUD-SPEC.md 第 4 节例外）。
func (s *SocialService) DeleteWithValidById(ctx context.Context, id int64) (bool, error) {
	affected, err := repository.NewSocialRepository(database.DB()).DeleteByID(ctx, id)
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// FindOwnerUserID 取某条绑定所属的用户ID，不存在时返回 ErrSocialNotFound。
// 供解绑接口校验归属用。
func (s *SocialService) FindOwnerUserID(ctx context.Context, id int64) (int64, error) {
	row, err := repository.NewSocialRepository(database.DB()).SelectByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrSocialNotFound) {
			return 0, ErrSocialNotFound
		}
		return 0, err
	}
	return row.UserID, nil
}

// buildSocialUpdateColumns 组装续绑时要覆盖的列。
//
// 令牌类字段一律写入（含空值）：三方重新授权后 refresh_token 等可能不再返回，
// 留着上一轮的旧值会让后续刷新令牌拿着废数据去请求。
// 不含 user_id/source：它们正是定位这条记录的依据，改它们等于换绑，不是本函数的语义。
func buildSocialUpdateColumns(b *bo.SysSocialBo) map[string]any {
	return map[string]any{
		"auth_id":            b.AuthID,
		"open_id":            b.OpenID,
		"user_name":          b.UserName,
		"nick_name":          b.NickName,
		"email":              b.Email,
		"avatar":             b.Avatar,
		"access_token":       b.AccessToken,
		"expire_in":          b.ExpireIn,
		"refresh_token":      b.RefreshToken,
		"access_code":        b.AccessCode,
		"union_id":           b.UnionID,
		"scope":              b.Scope,
		"token_type":         b.TokenType,
		"id_token":           b.IDToken,
		"mac_algorithm":      b.MacAlgorithm,
		"mac_key":            b.MacKey,
		"code":               b.Code,
		"oauth_token":        b.OauthToken,
		"oauth_token_secret": b.OauthTokenSecret,
	}
}

// acquireSocialLock 抢占某 authId 的绑定锁。ok 为 false 表示别人正持锁。
// 返回的 unlock 无论 ok 与否都可调用（未抢到时是空操作）。
func acquireSocialLock(ctx context.Context, authID string) (unlock func(), ok bool, err error) {
	key := socialRegisterLockKey + authID
	ok, err = redis.Client().SetNX(ctx, key, "", socialRegisterLockTTL).Result()
	if err != nil {
		return func() {}, false, fmt.Errorf("service: 获取三方绑定锁失败: %w", err)
	}
	if !ok {
		return func() {}, false, nil
	}
	return func() {
		// 用 WithoutCancel：响应发完请求 ctx 即取消，但锁必须释放，
		// 否则该 authId 要白等一个 TTL 才能再绑。
		if err := redis.Client().Del(context.WithoutCancel(ctx), key).Err(); err != nil {
			log.Printf("[system] 释放三方绑定锁 %s 失败: %v", key, err)
		}
	}, true, nil
}
