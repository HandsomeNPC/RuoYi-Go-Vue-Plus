package service

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/internal/system/model/bo"
	"ruoyi-go-vue-plus/internal/system/model/dto"
	"ruoyi-go-vue-plus/pkg/constant"
)

// TestBuildNoticeUpdateColumnsWritesEditableFields 可编辑字段一律写入，
// 使前端能把内容/备注清空——这正是编辑表单的本意。
func TestBuildNoticeUpdateColumnsWritesEditableFields(t *testing.T) {
	got := buildNoticeUpdateColumns(&bo.SysNoticeBo{
		NoticeID:      1761800000000000001,
		NoticeTitle:   "维护通知",
		NoticeContent: "",
		Remark:        "",
	})

	for _, col := range []string{"notice_title", "notice_content", "remark"} {
		if _, ok := got[col]; !ok {
			t.Errorf("%s 应一律写入（空值也要写），实际缺失", col)
		}
	}
	if got["notice_content"] != "" || got["remark"] != "" {
		t.Errorf("空值应原样写入，got %#v", got)
	}
}

// TestBuildNoticeUpdateColumnsSkipsBlankControlFields 控制字段缺省即不改。
//
// 漏传不该把线上的 notice_type='1'/status='0' 刷成空串——那会让公告
// 既不算通知也不算公告，前端字典渲染直接空白。
func TestBuildNoticeUpdateColumnsSkipsBlankControlFields(t *testing.T) {
	got := buildNoticeUpdateColumns(&bo.SysNoticeBo{
		NoticeID:    1761800000000000001,
		NoticeTitle: "维护通知",
	})

	for _, col := range []string{"notice_type", "status"} {
		if _, ok := got[col]; ok {
			t.Errorf("%s 为空时应跳过，实际写入了 %#v", col, got[col])
		}
	}
}

// TestBuildNoticeUpdateColumnsWritesPresentControlFields 控制字段有值时正常写入。
func TestBuildNoticeUpdateColumnsWritesPresentControlFields(t *testing.T) {
	got := buildNoticeUpdateColumns(&bo.SysNoticeBo{
		NoticeID:    1761800000000000001,
		NoticeTitle: "维护通知",
		NoticeType:  "2",
		Status:      "1",
	})

	if got["notice_type"] != "2" || got["status"] != "1" {
		t.Errorf("控制字段有值时应写入，got %#v", got)
	}
}

// TestBuildNoticeUpdateColumnsExcludesAuditAndKey 更新列不含审计字段与主键。
//
// 审计字段由 pkg/repository 的更新回调注入，手填会覆盖掉回调算出的值；
// 主键在 WHERE 里，写进 SET 等于允许改主键。
func TestBuildNoticeUpdateColumnsExcludesAuditAndKey(t *testing.T) {
	got := buildNoticeUpdateColumns(&bo.SysNoticeBo{
		NoticeID:    1761800000000000001,
		NoticeTitle: "维护通知",
	})

	for _, col := range []string{
		"notice_id", "create_by", "create_time", "create_dept", "update_by", "update_time",
	} {
		if _, ok := got[col]; ok {
			t.Errorf("更新列不应包含 %s，实际写入了 %#v", col, got[col])
		}
	}
}

// TestSupportsMessageBox 只有通用消息与通知公告入盒子，LLM 一概排除。
//
// LLM 是逐字流式下发的，每个片段都存一条会瞬间把 sys_message 写满。
func TestSupportsMessageBox(t *testing.T) {
	tests := []struct {
		name    string
		payload *dto.PushPayloadDTO
		want    bool
	}{
		{"nil", nil, false},
		{"通用消息", &dto.PushPayloadDTO{
			Type: constant.PushTypeMessage, Source: constant.PushSourceBackend}, true},
		{"通知公告", &dto.PushPayloadDTO{
			Type: constant.PushTypeNotice, Source: constant.PushSourceNotice}, true},
		{"自定义消息不入盒子", &dto.PushPayloadDTO{
			Type: constant.PushTypeCustom, Source: constant.PushSourceClient}, false},
		{"LLM 类型不入盒子", &dto.PushPayloadDTO{
			Type: constant.PushTypeLLM, Source: constant.PushSourceLLM}, false},
		// 类型合法但来源是 LLM：Java 侧同样排除，否则大模型走 message 类型就绕过了限制。
		{"LLM 来源不入盒子", &dto.PushPayloadDTO{
			Type: constant.PushTypeMessage, Source: constant.PushSourceLLM}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsMessageBox(tt.payload); got != tt.want {
				t.Errorf("supportsMessageBox() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestResolveCategory 分类按类型/来源推断（对齐 Java resolveCategory 的优先级）。
func TestResolveCategory(t *testing.T) {
	tests := []struct {
		name         string
		typ, source  string
		wantCategory string
		wantTitle    string
	}{
		{"公告类型", constant.PushTypeNotice, constant.PushSourceBackend,
			constant.MessageCategoryNotice, titleNotice},
		{"公告来源", constant.PushTypeMessage, constant.PushSourceNotice,
			constant.MessageCategoryNotice, titleNotice},
		{"工作流来源", constant.PushTypeMessage, constant.PushSourceWorkflow,
			constant.MessageCategoryWorkflow, titleWorkflow},
		{"其余归系统消息", constant.PushTypeMessage, constant.PushSourceBackend,
			constant.MessageCategorySystem, titleSystem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &dto.PushPayloadDTO{Type: tt.typ, Source: tt.source}
			if got := resolveCategory(payload); got != tt.wantCategory {
				t.Errorf("resolveCategory() = %q, 期望 %q", got, tt.wantCategory)
			}
			if got := resolveTitle(payload); got != tt.wantTitle {
				t.Errorf("resolveTitle() = %q, 期望 %q", got, tt.wantTitle)
			}
		})
	}
}

// TestJoinSendUserIDs 空列表表示全局广播，否则逗号拼接。
//
// 雪花 ID 必须按十进制原样拼，不能走科学计数法——那样 FIND_IN_SET 永远匹配不上。
func TestJoinSendUserIDs(t *testing.T) {
	if got := joinSendUserIDs(nil); got != constant.MessageGlobalUserIDs {
		t.Errorf("空列表应为全局标识 %q, got %q", constant.MessageGlobalUserIDs, got)
	}
	want := "1761100000000000001,1761100000000000002"
	got := joinSendUserIDs([]int64{1761100000000000001, 1761100000000000002})
	if got != want {
		t.Errorf("joinSendUserIDs() = %q, 期望 %q", got, want)
	}
}

// TestResolveContent 详细内容只从 map 形态 data 的 noticeContent 键取。
func TestResolveContent(t *testing.T) {
	tests := []struct {
		name string
		data any
		want string
	}{
		{"nil", nil, ""},
		{"取到内容", map[string]any{"noticeContent": "维护内容"}, "维护内容"},
		{"键缺失", map[string]any{"other": "x"}, ""},
		{"非 map 形态", "plain", ""},
		{"值非字符串", map[string]any{"noticeContent": 123}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveContent(&dto.PushPayloadDTO{Data: tt.data})
			if got != tt.want {
				t.Errorf("resolveContent() = %q, 期望 %q", got, tt.want)
			}
		})
	}
}

// TestParseMessageData 扩展数据解析：空串与坏 JSON 都按 nil 处理，不让整个盒子失败。
func TestParseMessageData(t *testing.T) {
	if got := parseMessageData(""); got != nil {
		t.Errorf("空串应返回 nil, got %#v", got)
	}
	if got := parseMessageData("   "); got != nil {
		t.Errorf("空白串应返回 nil, got %#v", got)
	}
	if got := parseMessageData("{bad json"); got != nil {
		t.Errorf("坏 JSON 应返回 nil（不阻断消息盒子）, got %#v", got)
	}

	got := parseMessageData(`{"noticeId":1}`)
	want := map[string]any{"noticeId": float64(1)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseMessageData() = %#v, 期望 %#v", got, want)
	}
}
