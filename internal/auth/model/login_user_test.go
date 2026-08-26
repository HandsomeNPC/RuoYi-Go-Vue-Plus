package model

import (
	"errors"
	"testing"
)

func TestLoginID(t *testing.T) {
	u := &LoginUser{UserID: 1761100000000000001, UserType: "sys_user"}

	id, err := u.LoginID()
	if err != nil {
		t.Fatalf("LoginID 应成功: %v", err)
	}
	if want := "sys_user:1761100000000000001"; id != want {
		t.Errorf("LoginID = %q, 期望 %q", id, want)
	}
}

func TestLoginIDRejectsIncomplete(t *testing.T) {
	cases := []struct {
		name string
		user *LoginUser
		want error
	}{
		{"nil", nil, ErrUserTypeEmpty},
		{"缺 userType", &LoginUser{UserID: 1}, ErrUserTypeEmpty},
		{"缺 userID", &LoginUser{UserType: "sys_user"}, ErrUserIDEmpty},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.user.LoginID()
			if !errors.Is(err, tt.want) {
				t.Errorf("LoginID err = %v, 期望 %v", err, tt.want)
			}
		})
	}
}
