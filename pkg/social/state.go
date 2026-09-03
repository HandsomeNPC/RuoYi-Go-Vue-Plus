package social

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/redis"
)

// stateTTL 授权超时时间，对齐 Java AuthRedisStateCache.cache 的默认三分钟。
const stateTTL = 3 * time.Minute

// ErrIllegalState state 缺失、已过期或已被用过。
// 对应 Java AuthChecker.checkState 抛的 ILLEGAL_STATUS(5009)。
var ErrIllegalState = errors.New("social: 三方登录 state 非法或已失效")

// newState 生成随机 state，对齐 Java AuthStateUtils.createState 的 UUID。
func newState() string {
	return uuid.NewString()
}

// stateKey 拼 Redis 键，对齐 Java GlobalConstants.SOCIAL_AUTH_CODE_KEY + state。
func stateKey(state string) string {
	return constant.SocialAuthCodeKey + state
}

// cacheState 把 state 存进 Redis，三分钟过期。
func cacheState(ctx context.Context, state string) error {
	if err := redis.Client().Set(ctx, stateKey(state), state, stateTTL).Err(); err != nil {
		return fmt.Errorf("social: 缓存 state 失败: %w", err)
	}
	return nil
}

// checkState 校验 state 是否有效，校验通过后立即删除使其单次有效。
//
// 与 Java 的有意差异：AuthRedisStateCache 只有 containsKey，校验后不删，
// 同一个 state 在三分钟内可反复重放——而 state 存在的全部意义就是防 CSRF。
// 这里用 GETDEL 原子地取值兼作废，并发重放只有一个能过。
func checkState(ctx context.Context, state string) error {
	if state == "" {
		return ErrIllegalState
	}
	err := redis.Client().GetDel(ctx, stateKey(state)).Err()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			// 键不存在：已过期、已用过，或 state 是伪造的。
			return ErrIllegalState
		}
		return fmt.Errorf("social: 校验 state 失败: %w", err)
	}
	return nil
}
