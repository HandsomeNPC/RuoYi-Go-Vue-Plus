package service

import (
	"testing"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/pkg/constant"
)

// TestBuildPostUpdateColumnsWritesEditableFields 可编辑字段一律写入，
// 使前端能把类别编码/备注清空——这正是编辑表单的本意。
func TestBuildPostUpdateColumnsWritesEditableFields(t *testing.T) {
	got := buildPostUpdateColumns(&bo.SysPostBo{
		PostID:       1761200000000000001,
		DeptID:       1761000000000000100,
		PostCode:     "ceo",
		PostName:     "董事长",
		PostCategory: "",
		PostSort:     1,
		Remark:       "",
	})

	for _, col := range []string{
		"dept_id", "post_code", "post_name", "post_category", "post_sort", "remark",
	} {
		if _, ok := got[col]; !ok {
			t.Errorf("%s 应一律写入（空值也要写），实际缺失", col)
		}
	}
	if got["post_category"] != "" || got["remark"] != "" {
		t.Errorf("空值应原样写入，got %#v", got)
	}
}

// TestBuildPostUpdateColumnsSkipsBlankStatus 状态缺省即视为不改，
// 漏传字段不该把线上的 '0' 刷成空串。
func TestBuildPostUpdateColumnsSkipsBlankStatus(t *testing.T) {
	got := buildPostUpdateColumns(&bo.SysPostBo{Status: ""})
	if _, ok := got["status"]; ok {
		t.Error("空状态应跳过，不该写入 status")
	}
}

// TestBuildPostUpdateColumnsWritesPresentStatus 传入状态时按值写入。
func TestBuildPostUpdateColumnsWritesPresentStatus(t *testing.T) {
	got := buildPostUpdateColumns(&bo.SysPostBo{Status: constant.StatusDisable})
	if got["status"] != constant.StatusDisable {
		t.Errorf("status = %v, 期望 %q", got["status"], constant.StatusDisable)
	}
}

// TestBuildPostUpdateColumnsExcludesAuditAndKey 审计字段与主键不该进更新列——
// update_by/update_time 由回调注入，post_id 由 Where 条件携带。
func TestBuildPostUpdateColumnsExcludesAuditAndKey(t *testing.T) {
	got := buildPostUpdateColumns(&bo.SysPostBo{PostID: 1761200000000000001})
	for _, col := range []string{"post_id", "create_by", "create_time", "update_by", "update_time", "del_flag"} {
		if _, ok := got[col]; ok {
			t.Errorf("%s 不该进更新列", col)
		}
	}
}

// TestBuildUserProfileColumnsOnlyNonEmpty 个人资料字段全可选：空串/0 视为未传，
// 不该把线上的昵称/邮箱刷成空串（对齐 Java setIfPresent）。
func TestBuildUserProfileColumnsOnlyNonEmpty(t *testing.T) {
	got := buildUserProfileColumns(&bo.SysUserProfileBo{
		NickName:    "",
		Email:       "",
		PhoneNumber: "",
		Gender:      "",
		Avatar:      0,
	})
	if len(got) != 0 {
		t.Errorf("全空入参不应产生更新列，got %#v", got)
	}
}

// TestBuildUserProfileColumnsWritesPresent 传入的字段按值写入，0 的 avatar 跳过。
func TestBuildUserProfileColumnsWritesPresent(t *testing.T) {
	got := buildUserProfileColumns(&bo.SysUserProfileBo{
		NickName:    "张三",
		Email:       "z@ex.com",
		PhoneNumber: "13800000000",
		Gender:      "0",
		Avatar:      0,
	})
	want := map[string]any{
		"nick_name":    "张三",
		"email":        "z@ex.com",
		"phone_number": "13800000000",
		"gender":       "0",
	}
	if len(got) != len(want) {
		t.Fatalf("更新列数 = %d, 期望 %d（avatar=0 应跳过）, got %#v", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, 期望 %v", k, got[k], v)
		}
	}
	if _, ok := got["avatar"]; ok {
		t.Error("avatar=0 应跳过")
	}
}
