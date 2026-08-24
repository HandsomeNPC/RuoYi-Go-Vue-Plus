package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

// newTestStore 起一个内存 Redis 并返回会话存储。
//
// 用 miniredis 而非真实 Redis：会话的 TTL 语义（滑动续期、过期即失效）
// 是本包最需要锁住的行为，而它们靠真实 Redis 测要么依赖外部环境、
// 要么得 sleep 等秒级过期。miniredis 支持 FastForward 直接推进时钟，
// 让「空闲超时」这件事能被确定性地断言。
func newTestStore(t *testing.T) (*SessionStore, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewSessionStore(rdb), mr
}

func testSession() *Session {
	return &Session{
		User: &LoginUser{
			UserID:   1761100000000000001,
			Username: "admin",
			UserType: UserTypeSys,
		},
		ActiveTimeout: 1800,
	}
}

// TestSessionSaveLoadRoundTrip 会话读写往返，含雪花 id 精度。
func TestSessionSaveLoadRoundTrip(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	want := testSession()
	if err := store.Save(ctx, "tok", want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Load(ctx, "tok")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.User.UserID != want.User.UserID {
		t.Errorf("UserID = %d, 期望 %d", got.User.UserID, want.User.UserID)
	}
	if got.User.Username != want.User.Username {
		t.Errorf("Username = %q, 期望 %q", got.User.Username, want.User.Username)
	}
	if got.ActiveTimeout != want.ActiveTimeout {
		t.Errorf("ActiveTimeout = %d, 期望 %d", got.ActiveTimeout, want.ActiveTimeout)
	}
}

// TestSessionSaveSetsActiveTimeoutTTL 会话的 TTL 必须取 ActiveTimeout ——
// 这是空闲超时的载体，不设就等于会话永不过期。
func TestSessionSaveSetsActiveTimeoutTTL(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "tok", testSession()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got, want := mr.TTL(TokenKeyPrefix+"tok"), 1800*time.Second; got != want {
		t.Errorf("TTL = %v, 期望 %v", got, want)
	}
}

// TestSessionExpiresAfterIdle 空闲超过 activeTimeout 后会话消失。
//
// 这正是「滑动空闲超时」要达成的效果：token 本身没过期（JWT 的 exp 是 7 天），
// 但服务端会话没了，鉴权在第 2 步被拦下。
func TestSessionExpiresAfterIdle(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "tok", testSession()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	mr.FastForward(1801 * time.Second)

	if _, err := store.Load(ctx, "tok"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("空闲超时后应返回 ErrSessionNotFound: 得到 %v", err)
	}
}

// TestSessionRenewSlidesTTL 续期把 TTL 重置回完整的 activeTimeout。
//
// 锁住「滑动」而非「固定」窗口：活跃用户不该在登录满 30 分钟后被登出。
func TestSessionRenewSlidesTTL(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "tok", testSession()); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 消耗掉一半空闲窗口。
	mr.FastForward(900 * time.Second)
	if got := mr.TTL(TokenKeyPrefix + "tok"); got != 900*time.Second {
		t.Fatalf("前提不成立: 推进后 TTL = %v, 期望 900s", got)
	}

	if err := store.Renew(ctx, "tok", 1800); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if got, want := mr.TTL(TokenKeyPrefix+"tok"), 1800*time.Second; got != want {
		t.Errorf("续期后 TTL = %v, 期望重置为 %v", got, want)
	}
}

// TestSessionDeleteRevokes 删除会话是 JWT 唯一的作废手段。
//
// token 本身签发后不可撤销，登出/踢下线全靠这一步 —— 若 Load 在会话删除后
// 仍能成功，登出就形同虚设。
func TestSessionDeleteRevokes(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "tok", testSession()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(ctx, "tok"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Load(ctx, "tok"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("删除后应返回 ErrSessionNotFound: 得到 %v", err)
	}
}

// TestSessionDeleteIsIdempotent 重复登出、或 token 已过期后再登出，都不该报错。
func TestSessionDeleteIsIdempotent(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.Delete(ctx, "never-existed"); err != nil {
		t.Errorf("删除不存在的会话不该报错: %v", err)
	}
	if err := store.Delete(ctx, ""); err != nil {
		t.Errorf("删除空 token 不该报错: %v", err)
	}
}

// TestSessionNeverExpireWhenTimeoutNonPositive ActiveTimeout <= 0 表示不过期。
//
// 对齐 sa-token 的 NEVER_EXPIRE(-1)。有意与 Java 的 PlusSaTokenDao 有一处偏差：
// 那边 timeout==0 是**静默丢弃**（根本不写），会造出「登录成功但立刻没会话」
// 这种无法自证的状态，不予复刻。
func TestSessionNeverExpireWhenTimeoutNonPositive(t *testing.T) {
	for _, timeout := range []int64{0, -1} {
		store, mr := newTestStore(t)
		ctx := context.Background()

		sess := testSession()
		sess.ActiveTimeout = timeout
		if err := store.Save(ctx, "tok", sess); err != nil {
			t.Fatalf("ActiveTimeout=%d Save: %v", timeout, err)
		}

		if ttl := mr.TTL(TokenKeyPrefix + "tok"); ttl != 0 {
			t.Errorf("ActiveTimeout=%d 应不设过期, 得到 TTL=%v", timeout, ttl)
		}
		// 关键：即便推进很久，会话依然在。
		mr.FastForward(365 * 24 * time.Hour)
		if _, err := store.Load(ctx, "tok"); err != nil {
			t.Errorf("ActiveTimeout=%d 的会话不该过期: %v", timeout, err)
		}
	}
}

// TestSessionRenewNoopWhenNeverExpire 不过期的会话无需续期，
// 且续期动作不能反而给它加上 TTL。
func TestSessionRenewNoopWhenNeverExpire(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	sess := testSession()
	sess.ActiveTimeout = -1
	if err := store.Save(ctx, "tok", sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Renew(ctx, "tok", -1); err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if ttl := mr.TTL(TokenKeyPrefix + "tok"); ttl != 0 {
		t.Errorf("续期不该给不过期的会话加上 TTL, 得到 %v", ttl)
	}
}

// TestSessionLoadCorruptedPayload Redis 里存着读不懂的数据时，
// 对调用方而言等同于「没有可用会话」，让用户重新登录即可。
func TestSessionLoadCorruptedPayload(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	mr.Set(TokenKeyPrefix+"tok", "{not-json")
	if _, err := store.Load(ctx, "tok"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("损坏的会话数据应返回 ErrSessionNotFound: 得到 %v", err)
	}

	// 合法 JSON 但没有 user 字段，同样不可用。
	mr.Set(TokenKeyPrefix+"tok2", `{"activeTimeout":1800}`)
	if _, err := store.Load(ctx, "tok2"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("缺 user 的会话应返回 ErrSessionNotFound: 得到 %v", err)
	}
}

// TestSessionSaveRejectsEmptyInput 空 token 或空内容是编程错误，应显式报错。
func TestSessionSaveRejectsEmptyInput(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "", testSession()); err == nil {
		t.Error("空 token 应报错")
	}
	if err := store.Save(ctx, "tok", nil); err == nil {
		t.Error("nil 会话应报错")
	}
	if err := store.Save(ctx, "tok", &Session{}); err == nil {
		t.Error("缺 User 的会话应报错")
	}
}

// TestSessionLoadDistinguishesRedisFailure Redis 故障必须与「会话不存在」区分。
//
// 混为一谈会让一次 Redis 抖动表现成「所有人被登出」，而日志里只有一片 401 ——
// 前者该告警并返回 500，后者是正常的登录态过期。
func TestSessionLoadDistinguishesRedisFailure(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	if err := store.Save(ctx, "tok", testSession()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	mr.Close() // 模拟 Redis 不可用

	_, err := store.Load(ctx, "tok")
	if err == nil {
		t.Fatal("Redis 不可用时应报错")
	}
	if errors.Is(err, ErrSessionNotFound) {
		t.Error("Redis 故障不该被当成「会话不存在」")
	}
}
