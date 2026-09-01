package handler

import (
	"reflect"
	"testing"
)

// TestParseIDs 逗号分隔的主键串切分。
func TestParseIDs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []int64
	}{
		{"单个", "1762000000000000001", []int64{1762000000000000001}},
		{"多个", "1,2,3", []int64{1, 2, 3}},
		{"含空白", " 1 , 2 ", []int64{1, 2}},
		// 超出 JS 安全整数范围的 ID 前端以字符串下发，ParseInt 不丢精度。
		{"雪花 ID 不丢精度", "1762000000000000001,1762000000000000002",
			[]int64{1762000000000000001, 1762000000000000002}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIDs(tt.in)
			if err != nil {
				t.Fatalf("parseIDs(%q): %v", tt.in, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIDs(%q) = %v, 期望 %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseIDsRejects 任一段非法即整体拒绝——静默丢弃会删成部分成功。
func TestParseIDsRejects(t *testing.T) {
	for _, in := range []string{
		"",                     // 空串
		",",                    // 只有分隔符
		"1,",                   // 尾随分隔符
		"1,abc",                // 混入非数字
		"1,0",                  // 0 非法主键
		"1,-2",                 // 负数
		"1,2;DROP x",           // 注入形态
		"99999999999999999999", // 溢出 int64
	} {
		t.Run(in, func(t *testing.T) {
			if got, err := parseIDs(in); err == nil {
				t.Errorf("parseIDs(%q) = %v, 应返回错误", in, got)
			}
		})
	}
}
