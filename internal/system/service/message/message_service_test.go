package message

import (
	"reflect"
	"testing"

	pushsvc "ruoyi-go-vue-plus/internal/resource/push"
	syscons "ruoyi-go-vue-plus/internal/system/constant"
)

// TestSupportsMessageBox 只有通用消息与通知公告入盒子，LLM 一概排除。
//
// LLM 是逐字流式下发的，每个片段都存一条会瞬间把 sys_message 写满。
func TestSupportsMessageBox(t *testing.T) {
	tests := []struct {
		name    string
		payload *pushsvc.PayloadDTO
		want    bool
	}{
		{"nil", nil, false},
		{"通用消息", &pushsvc.PayloadDTO{
			Type: pushsvc.TypeMessage, Source: pushsvc.SourceBackend}, true},
		{"通知公告", &pushsvc.PayloadDTO{
			Type: pushsvc.TypeNotice, Source: pushsvc.SourceNotice}, true},
		{"自定义消息不入盒子", &pushsvc.PayloadDTO{
			Type: pushsvc.TypeCustom, Source: pushsvc.SourceClient}, false},
		{"LLM 类型不入盒子", &pushsvc.PayloadDTO{
			Type: pushsvc.TypeLLM, Source: pushsvc.SourceLLM}, false},
		// 类型合法但来源是 LLM：同样排除，否则大模型走 message 类型就绕过了限制。
		{"LLM 来源不入盒子", &pushsvc.PayloadDTO{
			Type: pushsvc.TypeMessage, Source: pushsvc.SourceLLM}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := supportsMessageBox(tt.payload); got != tt.want {
				t.Errorf("supportsMessageBox() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

// TestResolveCategory 分类按类型/来源推断。
func TestResolveCategory(t *testing.T) {
	tests := []struct {
		name         string
		typ, source  string
		wantCategory string
		wantTitle    string
	}{
		{"公告类型", pushsvc.TypeNotice, pushsvc.SourceBackend,
			syscons.MessageCategoryNotice, titleNotice},
		{"公告来源", pushsvc.TypeMessage, pushsvc.SourceNotice,
			syscons.MessageCategoryNotice, titleNotice},
		{"工作流来源", pushsvc.TypeMessage, pushsvc.SourceWorkflow,
			syscons.MessageCategoryWorkflow, titleWorkflow},
		{"其余归系统消息", pushsvc.TypeMessage, pushsvc.SourceBackend,
			syscons.MessageCategorySystem, titleSystem},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := &pushsvc.PayloadDTO{Type: tt.typ, Source: tt.source}
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
// 雪花 ID 必须按十进制原样拼，不能走科学计数法——那样接收人整段比对永远匹配不上。
func TestJoinSendUserIDs(t *testing.T) {
	if got := joinSendUserIDs(nil); got != syscons.MessageGlobalUserIDs {
		t.Errorf("空列表应为全局标识 %q, got %q", syscons.MessageGlobalUserIDs, got)
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
			got := resolveContent(&pushsvc.PayloadDTO{Data: tt.data})
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
