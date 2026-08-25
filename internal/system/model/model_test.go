package model

import (
	"reflect"
	"testing"
)

// TestTableNames 表名必须与原项目一致。
func TestTableNames(t *testing.T) {
	if got, want := (SysUser{}).TableName(), "sys_user"; got != want {
		t.Errorf("SysUser.TableName() = %q, 期望 %q", got, want)
	}
	if got, want := (SysClient{}).TableName(), "sys_client"; got != want {
		t.Errorf("SysClient.TableName() = %q, 期望 %q", got, want)
	}
}

// TestPasswordAndSecretNeverSerialized 锁住密码与客户端密钥不会漏进响应体。
func TestPasswordAndSecretNeverSerialized(t *testing.T) {
	userField, ok := reflect.TypeOf(SysUser{}).FieldByName("Password")
	if !ok {
		t.Fatal("SysUser 没有 Password 字段")
	}
	if got := userField.Tag.Get("json"); got != "-" {
		t.Errorf(`SysUser.Password 的 json 标签 = %q, 必须为 "-"(否则密码哈希会漏进响应体)`, got)
	}

	clientField, ok := reflect.TypeOf(SysClient{}).FieldByName("ClientSecret")
	if !ok {
		t.Fatal("SysClient 没有 ClientSecret 字段")
	}
	if got := clientField.Tag.Get("json"); got != "-" {
		t.Errorf(`SysClient.ClientSecret 的 json 标签 = %q, 必须为 "-"(否则密码哈希会漏进响应体)`, got)
	}
}
