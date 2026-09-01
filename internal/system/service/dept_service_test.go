package service

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/internal/system/model/bo"
)

// TestBuildDeptUpdateColumns 可编辑字段一律写（让前端能清空联系方式），
// 状态缺省则跳过（漏传不该把线上的 '0' 刷成空串）。
func TestBuildDeptUpdateColumns(t *testing.T) {
	tests := []struct {
		name        string
		in          *bo.SysDeptBo
		wantPresent map[string]any
		wantAbsent  []string
	}{
		{
			name: "全字段",
			in: &bo.SysDeptBo{
				ParentID:     1761000000000000101,
				DeptName:     "研发部门",
				DeptCategory: "RD",
				OrderNum:     3,
				Leader:       1761100000000000001,
				Phone:        "15888888888",
				Email:        "rd@qq.com",
				Status:       "0",
			},
			wantPresent: map[string]any{
				"parent_id":     int64(1761000000000000101),
				"dept_name":     "研发部门",
				"dept_category": "RD",
				"order_num":     3,
				"leader":        int64(1761100000000000001),
				"phone":         "15888888888",
				"email":         "rd@qq.com",
				"status":        "0",
			},
		},
		{
			// 清空联系方式是编辑表单的合法意图，空串必须写进库。
			name: "清空联系方式",
			in: &bo.SysDeptBo{
				DeptName: "研发部门",
				Phone:    "",
				Email:    "",
				Status:   "1",
			},
			wantPresent: map[string]any{
				"phone":  "",
				"email":  "",
				"status": "1",
			},
		},
		{
			// 状态缺省视为不改，不能落进更新列。
			name: "状态缺省",
			in: &bo.SysDeptBo{
				DeptName: "研发部门",
				Status:   "",
			},
			wantAbsent: []string{"status"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDeptUpdateColumns(tt.in)

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
			// ancestors 由调用方按新旧父级另行补上，不该出现在这里。
			if _, ok := got["ancestors"]; ok {
				t.Error("更新列不应含 ancestors")
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

// TestParseAncestorIDs 根部门的祖先链是 "0"，那不是真实部门，必须过滤掉——
// 否则启用祖先链时会去 UPDATE 一个不存在的主键。
func TestParseAncestorIDs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []int64
	}{
		{"根祖先链", "0", []int64{}},
		{"单级", "0,1761000000000000100", []int64{1761000000000000100}},
		{"多级", "0,1761000000000000100,1761000000000000101",
			[]int64{1761000000000000100, 1761000000000000101}},
		{"空串", "", []int64{}},
		{"含空白", "0, 1761000000000000100 ", []int64{1761000000000000100}},
		{"混入非数字", "0,abc,1761000000000000100", []int64{1761000000000000100}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAncestorIDs(tt.in)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAncestorIDs(%q) = %v, 期望 %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestContainsAncestor 按整段比对而非子串匹配：主键 100 不该命中祖先链里的 1001。
func TestContainsAncestor(t *testing.T) {
	tests := []struct {
		name      string
		ancestors string
		deptID    int64
		want      bool
	}{
		{"命中中间段", "0,100,1001", 100, true},
		{"命中末段", "0,100,1001", 1001, true},
		{"命中根 0", "0,100", 0, true},
		{"未命中", "0,100,1001", 200, false},
		// 子串匹配会把 10 误判成命中 100/1001，这正是不能用 strings.Contains 的原因。
		{"前缀不算命中", "0,100,1001", 10, false},
		{"空祖先链", "", 100, false},
		{"含空白", "0, 100 ,1001", 100, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsAncestor(tt.ancestors, tt.deptID); got != tt.want {
				t.Errorf("containsAncestor(%q, %d) = %v, 期望 %v",
					tt.ancestors, tt.deptID, got, tt.want)
			}
		})
	}
}

// TestDeptCacheKey 缓存 key 是裸十进制主键，与 Java @Cacheable(key = "#deptId") 同形。
func TestDeptCacheKey(t *testing.T) {
	if got, want := deptCacheKey(1761000000000000100), "1761000000000000100"; got != want {
		t.Errorf("deptCacheKey = %q, 期望 %q", got, want)
	}
}
