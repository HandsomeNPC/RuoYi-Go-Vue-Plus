package service

import (
	"testing"

	"ruoyi-go-vue-plus/pkg/constant"
)

// TestMapLoginStatus 操作类型 → sys_login_info.status 落表值的映射，
// 对应 recordLoginInfo 的 if/else if 分支。
func TestMapLoginStatus(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{"登录成功记 0", constant.ConstantLoginSuccess, constant.ConstantSuccess},
		{"退出记 0", constant.ConstantLogout, constant.ConstantSuccess},
		{"注册记 0", constant.ConstantRegister, constant.ConstantSuccess},
		{"登录失败记 1", constant.ConstantLoginFail, constant.ConstantFail},
		// 两个分支都不命中时 status 留 null。
		{"未识别取值留空", "SomethingElse", ""},
		{"空串留空", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mapLoginStatus(c.status); got != c.want {
				t.Errorf("mapLoginStatus(%q) = %q, want %q", c.status, got, c.want)
			}
		})
	}
}
