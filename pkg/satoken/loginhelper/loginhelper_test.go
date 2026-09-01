package loginhelper

import (
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"ruoyi-go-vue-plus/pkg/constant"
	authmodel "ruoyi-go-vue-plus/pkg/model"
)

// TestToSaTokenPerms 超管的 *:*:* 必须换成 *，其余权限码原样透传。
//
// 这条转换是超管能否通过鉴权的唯一依赖：sa-token-go 的 matchPermission 对 *:*:*
// 会先命中「以 :* 结尾」的前缀分支并提前返回 false，走不到按段比对，故超管处处被拒。
func TestToSaTokenPerms(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"超管全权限换成单星", []string{constant.AllPermission}, []string{"*"}},
		{"普通权限码原样", []string{"system:client:list"}, []string{"system:client:list"}},
		{"前缀通配原样", []string{"system:*"}, []string{"system:*"}},
		{"混杂时只换全权限", []string{"demo:demo:list", constant.AllPermission},
			[]string{"demo:demo:list", "*"}},
		// 形似但不等于 *:*:* 的不动，避免过度匹配。
		{"两段星号原样", []string{"*:*"}, []string{"*:*"}},
		{"空集合", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toSaTokenPerms(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toSaTokenPerms(%v) = %v, 期望 %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestAllPermissionLiteral 前端 hasPermi 按字面量比对，常量值不可改动。
func TestAllPermissionLiteral(t *testing.T) {
	if constant.AllPermission != "*:*:*" {
		t.Errorf("AllPermission = %q, 前端契约要求 *:*:*", constant.AllPermission)
	}
}

// newTestCtx 造一个带 request 的 gin.Context，不挂 TokenInterceptor（故取不到 token）。
func newTestCtx() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c
}

// TestGetLoginUserCachesHit 命中缓存时直接返回快照，不再解会话。
//
// 一个请求里 AuditContext、oplog、handler 各要取一次登录用户，不缓存就是三次 Redis 往返。
// 这里预置缓存来断言「有缓存就不回源」——无 Redis 环境下回源必得 nil，故拿到非 nil 即证明走的是缓存。
func TestGetLoginUserCachesHit(t *testing.T) {
	c := newTestCtx()
	want := &authmodel.LoginUser{UserID: 1, DeptID: 100}
	c.Set(loginUserCacheKey, want)

	if got := GetLoginUser(c); got != want {
		t.Errorf("GetLoginUser() = %v, 期望复用缓存里的同一指针 %v", got, want)
	}
	// 派生的取值函数同样应吃到缓存。
	if got := GetDeptID(c); got != 100 {
		t.Errorf("GetDeptID() = %d, 期望 100", got)
	}
}

// TestGetLoginUserCachesMiss 未登录也要落缓存，否则匿名请求每个调用点都白跑一次 Redis。
func TestGetLoginUserCachesMiss(t *testing.T) {
	c := newTestCtx()
	if got := GetLoginUser(c); got != nil {
		t.Fatalf("GetLoginUser() = %v, 无 token 时期望 nil", got)
	}
	v, ok := c.Get(loginUserCacheKey)
	if !ok {
		t.Fatal("未登录时也应写入缓存键，否则后续调用点会重复回源")
	}
	if lu, _ := v.(*authmodel.LoginUser); lu != nil {
		t.Errorf("缓存值 = %v, 期望 nil", lu)
	}
	// 第二次仍返回 nil，且不 panic。
	if got := GetLoginUser(c); got != nil {
		t.Errorf("第二次 GetLoginUser() = %v, 期望 nil", got)
	}
}
