package loginhelper

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/pkg/constant"
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
