package service

import (
	"testing"

	"ruoyi-go-vue-plus/internal/system/model/bo"
)

// TestBuildConfigUpdateColumnsAlwaysWritesEditable 名称/键名/值/备注一律写入，
// 让前端能把备注清空——用 Updates(struct) 会跳过零值，把空串当成"未修改"而丢弃。
func TestBuildConfigUpdateColumnsAlwaysWritesEditable(t *testing.T) {
	got := buildConfigUpdateColumns(&bo.SysConfigBo{
		ConfigID:    1,
		ConfigName:  "n",
		ConfigKey:   "k",
		ConfigValue: "v",
		Remark:      "", // 清空备注
	})

	if _, ok := got["remark"]; !ok {
		t.Error("remark 为空串时也须写入，否则备注清不掉")
	}
	for col, want := range map[string]any{
		"config_name":  "n",
		"config_key":   "k",
		"config_value": "v",
		"remark":       "",
	} {
		if got[col] != want {
			t.Errorf("列 %s = %v, 期望 %v", col, got[col], want)
		}
	}
}

// TestBuildConfigUpdateColumnsSkipsBlankConfigType configType 缺省即不改：
// 漏传字段不该把线上的 'Y' 刷成空串，那会让内置参数失去删除保护。
func TestBuildConfigUpdateColumnsSkipsBlankConfigType(t *testing.T) {
	got := buildConfigUpdateColumns(&bo.SysConfigBo{ConfigID: 1, ConfigType: ""})
	if _, ok := got["config_type"]; ok {
		t.Error("configType 为空串时不应出现在更新列里")
	}

	got = buildConfigUpdateColumns(&bo.SysConfigBo{ConfigID: 1, ConfigType: "Y"})
	if got["config_type"] != "Y" {
		t.Errorf("config_type = %v, 期望 Y", got["config_type"])
	}
}

// TestBuildConfigUpdateColumnsOmitsAuditAndID 主键与审计列不进更新列：
// config_id 是 WHERE 条件不该被 SET，update_by/update_time 由 pkg/repository 的回调补齐。
func TestBuildConfigUpdateColumnsOmitsAuditAndID(t *testing.T) {
	got := buildConfigUpdateColumns(&bo.SysConfigBo{ConfigID: 1, ConfigName: "n"})
	for _, col := range []string{"config_id", "create_by", "create_time", "update_by", "update_time"} {
		if _, ok := got[col]; ok {
			t.Errorf("更新列不应包含 %s", col)
		}
	}
}
