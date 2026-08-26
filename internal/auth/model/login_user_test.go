package model

import (
	"testing"

	"ruoyi-go-vue-plus/pkg/constant"
)

func TestLoginIDRoundTrip(t *testing.T) {
	u := &LoginUser{UserID: 1761100000000000001, UserType: UserTypeSys}

	id, ok := u.LoginID()
	if !ok {
		t.Fatal("LoginID 应成功")
	}
	if want := "sys_user:1761100000000000001"; id != want {
		t.Errorf("LoginID = %q, 期望 %q", id, want)
	}

	userType, userID, ok := ParseLoginID(id)
	if !ok {
		t.Fatal("ParseLoginID 应成功")
	}
	if userType != UserTypeSys || userID != u.UserID {
		t.Errorf("ParseLoginID = (%q, %d), 期望 (%q, %d)",
			userType, userID, UserTypeSys, u.UserID)
	}
}

func TestLoginIDRejectsIncomplete(t *testing.T) {
	cases := []struct {
		name string
		user *LoginUser
	}{
		{"nil", nil},
		{"缺 userType", &LoginUser{UserID: 1}},
		{"缺 userID", &LoginUser{UserType: UserTypeSys}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := tt.user.LoginID(); ok {
				t.Error("应返回 ok=false")
			}
		})
	}
}

func TestParseLoginIDRejectsMalformed(t *testing.T) {
	for _, in := range []string{"", "sys_user", "sys_user:", ":1", "sys_user:abc", "sys_user:0"} {
		if _, _, ok := ParseLoginID(in); ok {
			t.Errorf("ParseLoginID(%q) 应失败", in)
		}
	}
}

func TestParseLoginIDSplitsOnFirstColon(t *testing.T) {
	userType, _, ok := ParseLoginID("sys_user:123:extra")
	if ok {
		t.Errorf("ID 段非纯数字应失败, 却解析出 userType=%q", userType)
	}
}

func TestSuperAdminIDMatchesConstant(t *testing.T) {
	if superAdminUserID != constant.SuperAdminUserID {
		t.Errorf("model.superAdminUserID(%d) 与 constant.SuperAdminUserID(%d) 不一致",
			superAdminUserID, constant.SuperAdminUserID)
	}

	u := &LoginUser{UserID: constant.SuperAdminUserID}
	if !u.IsSuperAdmin() {
		t.Error("超管 id 应判定为超级管理员")
	}
	if (&LoginUser{UserID: 2}).IsSuperAdmin() {
		t.Error("非超管 id 不应判定为超级管理员")
	}
}
