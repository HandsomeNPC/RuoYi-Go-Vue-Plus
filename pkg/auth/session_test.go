package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	authmodel "ruoyi-go-vue-plus/internal/auth/model"
)

func newTestStore(t *testing.T) (*SessionStore, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	return NewSessionStore(rdb), mr
}

func testSession() *Session {
	return &Session{
		User: &authmodel.LoginUser{
			UserID:   1761100000000000001,
			Username: "admin",
			UserType: authmodel.UserTypeSys,
		},
		ActiveTimeout: 1800,
	}
}

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
		mr.FastForward(365 * 24 * time.Hour)
		if _, err := store.Load(ctx, "tok"); err != nil {
			t.Errorf("ActiveTimeout=%d 的会话不该过期: %v", timeout, err)
		}
	}
}

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

func TestSessionLoadCorruptedPayload(t *testing.T) {
	store, mr := newTestStore(t)
	ctx := context.Background()

	mr.Set(TokenKeyPrefix+"tok", "{not-json")
	if _, err := store.Load(ctx, "tok"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("损坏的会话数据应返回 ErrSessionNotFound: 得到 %v", err)
	}

	mr.Set(TokenKeyPrefix+"tok2", `{"activeTimeout":1800}`)
	if _, err := store.Load(ctx, "tok2"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("缺 user 的会话应返回 ErrSessionNotFound: 得到 %v", err)
	}
}

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
