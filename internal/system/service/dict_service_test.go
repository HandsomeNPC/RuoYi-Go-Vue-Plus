package service

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/internal/system/model/bo"
)

// TestBuildDictDataUpdateColumns 样式与备注一律写（让前端能清空），
// is_default 缺省则跳过（前端编辑表单不含该字段，漏传不该把 'Y' 刷成空串）。
func TestBuildDictDataUpdateColumns(t *testing.T) {
	tests := []struct {
		name        string
		in          *bo.SysDictDataBo
		wantPresent map[string]any
		wantAbsent  []string
	}{
		{
			name: "全字段",
			in: &bo.SysDictDataBo{
				DictSort:  3,
				DictLabel: "男",
				DictValue: "0",
				DictType:  "sys_user_gender",
				CssClass:  "bg-blue",
				ListClass: "primary",
				IsDefault: "Y",
				Remark:    "性别男",
			},
			wantPresent: map[string]any{
				"dict_sort":  3,
				"dict_label": "男",
				"dict_value": "0",
				"dict_type":  "sys_user_gender",
				"css_class":  "bg-blue",
				"list_class": "primary",
				"is_default": "Y",
				"remark":     "性别男",
			},
		},
		{
			// 清空样式与备注是编辑表单的合法意图，空串必须写进库。
			name: "清空样式与备注",
			in: &bo.SysDictDataBo{
				DictLabel: "男",
				CssClass:  "",
				ListClass: "",
				Remark:    "",
				IsDefault: "N",
			},
			wantPresent: map[string]any{
				"css_class":  "",
				"list_class": "",
				"remark":     "",
				"is_default": "N",
			},
		},
		{
			// is_default 缺省视为不改，不能落进更新列。
			name: "是否默认缺省",
			in: &bo.SysDictDataBo{
				DictLabel: "男",
				IsDefault: "",
			},
			wantAbsent: []string{"is_default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDictDataUpdateColumns(tt.in)

			for k, want := range tt.wantPresent {
				v, ok := got[k]
				if !ok {
					t.Errorf("更新列缺少 %q", k)
					continue
				}
				if !reflect.DeepEqual(v, want) {
					t.Errorf("更新列 %q = %#v, 期望 %#v", k, v, want)
				}
			}
			for _, k := range tt.wantAbsent {
				if _, ok := got[k]; ok {
					t.Errorf("更新列不应含 %q", k)
				}
			}
			// 审计字段由 pkg/repository 的回调注入。
			for _, k := range []string{"update_by", "update_time", "create_by", "create_time"} {
				if _, ok := got[k]; ok {
					t.Errorf("更新列不应含审计字段 %q", k)
				}
			}
		})
	}
}

// TestBuildDictTypeUpdateColumns 备注一律写，让前端能清空。
func TestBuildDictTypeUpdateColumns(t *testing.T) {
	got := buildDictTypeUpdateColumns(&bo.SysDictTypeBo{
		DictName: "用户性别",
		DictType: "sys_user_gender",
		Remark:   "",
	})

	want := map[string]any{
		"dict_name": "用户性别",
		"dict_type": "sys_user_gender",
		"remark":    "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("更新列 = %#v, 期望 %#v", got, want)
	}
}

// TestValidateDictTypeFormat 字典类型是 sys_dict_data 的关联键，
// 格式失守会让前端按类型取字典时静默取空——必须挡在写入之前。
func TestValidateDictTypeFormat(t *testing.T) {
	valid := []string{"sys_user_gender", "a", "a1_", "sys_normal_disable"}
	for _, s := range valid {
		t.Run("valid/"+s, func(t *testing.T) {
			if err := validateDictTypeFormat(s); err != nil {
				t.Errorf("validateDictTypeFormat(%q) = %v, 期望通过", s, err)
			}
		})
	}

	bad := []string{
		"",             // 空串
		"1abc",         // 数字开头
		"Sys_user",     // 含大写
		"sys-user",     // 含连字符
		"sys user",     // 含空格
		"x sys_user y", // 前后有内容（验证正则是整串锚定的）
		"sys_user\n",   // 尾随换行
	}
	for _, s := range bad {
		t.Run("bad/"+s, func(t *testing.T) {
			if err := validateDictTypeFormat(s); err == nil {
				t.Errorf("validateDictTypeFormat(%q) = nil, 应拒绝", s)
			}
		})
	}
}
