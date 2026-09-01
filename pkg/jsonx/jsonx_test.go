package jsonx

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	ginjson "github.com/gin-gonic/gin/codec/json"
)

// idHolder 覆盖三类字段：裸 int64、int64 切片、omitempty 的 int64。
// int64 编解码器由包 init 注册，无需在测试里显式调 Init。
type idHolder struct {
	ID       int64   `json:"id"`
	ParentID int64   `json:"parentId"`
	RoleIDs  []int64 `json:"roleIds"`
	Timeout  int64   `json:"timeout,omitempty"`
}

// TestEncodeByValue 形态只由值决定：超出 JS 安全整数范围才转字符串。
func TestEncodeByValue(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string // id 字段序列化后的片段
	}{
		{"雪花主键转字符串", 1762000000000000001, `"id":"1762000000000000001"`},
		{"正边界内仍是数字", maxSafeInteger, `"id":9007199254740991`},
		{"正边界外一位转字符串", maxSafeInteger + 1, `"id":"9007199254740992"`},
		{"负边界内仍是数字", minSafeInteger, `"id":-9007199254740991`},
		{"负边界外一位转字符串", minSafeInteger - 1, `"id":"-9007199254740992"`},
		// 根节点 parentId=0 必须保持数字：前端按 !== 0 严格比较判断有无上级。
		{"零值仍是数字", 0, `"id":0`},
		// activeTimeout=1800 / timeout=604800 这类普通数值不该被波及。
		{"普通数值仍是数字", 1800, `"id":1800`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Marshal(idHolder{ID: tt.in})
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if !strings.Contains(string(got), tt.want) {
				t.Errorf("Marshal(%d) = %s\n应包含 %s", tt.in, got, tt.want)
			}
		})
	}
}

// TestEncodeSliceElements 切片元素同样按值判断，逐个生效。
func TestEncodeSliceElements(t *testing.T) {
	got, err := Marshal(idHolder{RoleIDs: []int64{1761300000000000001, 0, 1800}})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `"roleIds":["1761300000000000001",0,1800]`
	if !strings.Contains(string(got), want) {
		t.Errorf("Marshal = %s\n应包含 %s", got, want)
	}
}

// TestEncodeOmitEmptyUnchanged omitempty 语义不能被自定义编码器改掉。
func TestEncodeOmitEmptyUnchanged(t *testing.T) {
	got, err := Marshal(idHolder{Timeout: 0})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(got), "timeout") {
		t.Errorf("Marshal = %s\ntimeout 为 0 且带 omitempty，应被省略", got)
	}

	got, err = Marshal(idHolder{Timeout: 604800})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(got), `"timeout":604800`) {
		t.Errorf("Marshal = %s\n非零时应输出", got)
	}
}

// TestDecodeAcceptsBothShapes 出参把大 id 变成字符串后，前端会原样回传字符串，
// 故入参必须同时认字符串与数字，否则编辑/删除直接 400。
func TestDecodeAcceptsBothShapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int64
	}{
		{"字符串形态", `{"id":"1762000000000000001"}`, 1762000000000000001},
		{"数字形态", `{"id":1762000000000000001}`, 1762000000000000001},
		{"小数值字符串", `{"id":"1800"}`, 1800},
		{"小数值数字", `{"id":1800}`, 1800},
		{"负值字符串", `{"id":"-9007199254740992"}`, -9007199254740992},
		{"null 保持零值", `{"id":null}`, 0},
		{"字段缺失保持零值", `{}`, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got idHolder
			if err := api.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("Unmarshal(%s): %v", tt.in, err)
			}
			if got.ID != tt.want {
				t.Errorf("Unmarshal(%s).ID = %d, 期望 %d", tt.in, got.ID, tt.want)
			}
		})
	}
}

// TestDecodeSliceBothShapes roleIds/postIds 前端声明为 string[]，混合形态也要能收。
func TestDecodeSliceBothShapes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []int64
	}{
		{"字符串数组", `{"roleIds":["1761300000000000001","1761300000000000002"]}`,
			[]int64{1761300000000000001, 1761300000000000002}},
		{"数字数组", `{"roleIds":[1761300000000000001,1761300000000000002]}`,
			[]int64{1761300000000000001, 1761300000000000002}},
		{"混合数组", `{"roleIds":["1761300000000000001",1800]}`,
			[]int64{1761300000000000001, 1800}},
		{"null 得 nil", `{"roleIds":null}`, nil},
		{"空数组", `{"roleIds":[]}`, []int64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got idHolder
			if err := api.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("Unmarshal(%s): %v", tt.in, err)
			}
			if !reflect.DeepEqual(got.RoleIDs, tt.want) {
				t.Errorf("Unmarshal(%s).RoleIDs = %v, 期望 %v", tt.in, got.RoleIDs, tt.want)
			}
		})
	}
}

// TestDecodeRejectsNonNumericString 非数字串必须报错，不能静默当零值放过去。
func TestDecodeRejectsNonNumericString(t *testing.T) {
	var got idHolder
	err := api.Unmarshal([]byte(`{"id":"abc"}`), &got)
	if err == nil {
		t.Fatal("非数字字符串应返回错误")
	}
	if !strings.Contains(err.Error(), "abc") {
		t.Errorf("错误信息应含原值便于定位, got %v", err)
	}
}

// TestRoundTrip 出参转成字符串后再回传，须还原成同一个值——这正是前端编辑的实际路径。
func TestRoundTrip(t *testing.T) {
	src := idHolder{ID: 1762000000000000001, ParentID: 0, RoleIDs: []int64{1761300000000000001}}

	body, err := Marshal(src)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back idHolder
	if err := api.Unmarshal(body, &back); err != nil {
		t.Fatalf("Unmarshal(%s): %v", body, err)
	}
	if !reflect.DeepEqual(src, back) {
		t.Errorf("往返后 = %+v, 期望 %+v (中间态 %s)", back, src, body)
	}
}

// TestNonInt64Unaffected 换 codec 只应改变 int64 形态，其余类型须与标准库逐字节一致。
func TestNonInt64Unaffected(t *testing.T) {
	type other struct {
		Str   string         `json:"str"`
		Int   int            `json:"int"`
		Int32 int32          `json:"int32"`
		Float float64        `json:"float"`
		Bool  bool           `json:"bool"`
		Map   map[string]int `json:"map"`
		Ptr   *string        `json:"ptr"`
		HTML  string         `json:"html"`
	}
	s := "x"
	in := other{
		Str: "a", Int: 1, Int32: 2, Float: 1.5, Bool: true,
		// map 键序：ConfigCompatibleWithStandardLibrary 开了 SortMapKeys，与标准库一致。
		Map: map[string]int{"b": 2, "a": 1}, Ptr: &s,
		// HTML 转义也须保持标准库行为。
		HTML: "<script>&",
	}

	want, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("标准库 Marshal: %v", err)
	}
	got, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("与标准库不一致:\ngot  %s\nwant %s", got, want)
	}
}

// TestIntNotAffected int 是独立类型，注册 int64 不应波及它（64 位平台上底层同宽）。
// 这条锁住「只拦 int64」的边界：若误注册成按 Kind 匹配，orderNum 这类 int 字段会跟着变形。
func TestIntNotAffected(t *testing.T) {
	type withInt struct {
		OrderNum int `json:"orderNum"`
	}
	// 取一个超安全范围的值：若 int 被误拦，它会变成字符串。
	got, err := Marshal(withInt{OrderNum: maxSafeInteger + 1})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(got), `"9007199254740992"`) {
		t.Errorf("Marshal = %s\nint 不应被 int64 编码器拦截", got)
	}
}

// TestRegisteredAtPackageInit 注册必须发生在包 init，不能挪进 Init()。
//
// jsoniter 按类型缓存 encoder（frozenConfig.encoderCache），某类型一旦被编码过其
// encoder 即固化，之后再 RegisterTypeEncoder 也挤不掉。若把注册挪进 Init()，
// 任何在 Init() 之前发生的 Marshal 都会把该类型钉死成裸数字形态。
// 本用例故意不调 Init()，直接编码就应看到字符串形态。
func TestRegisteredAtPackageInit(t *testing.T) {
	got, err := Marshal(idHolder{ID: 1762000000000000001})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(got), `"id":"1762000000000000001"`) {
		t.Errorf("Marshal = %s\n未调 Init 也应生效(注册在包 init)", got)
	}
}

// TestInitTakesOverGinAPI Init 须把 gin 的全局 codec 换成本包实现，
// 因为 c.JSON 与参数绑定走的都是 ginjson.API。
func TestInitTakesOverGinAPI(t *testing.T) {
	Init()

	got, err := ginjson.API.Marshal(idHolder{ID: 1762000000000000001})
	if err != nil {
		t.Fatalf("ginjson.API.Marshal: %v", err)
	}
	if !strings.Contains(string(got), `"id":"1762000000000000001"`) {
		t.Errorf("ginjson.API.Marshal = %s\nInit 后应由本包接管", got)
	}

	var back idHolder
	if err := ginjson.API.Unmarshal([]byte(`{"id":"1762000000000000001"}`), &back); err != nil {
		t.Fatalf("ginjson.API.Unmarshal: %v", err)
	}
	if back.ID != 1762000000000000001 {
		t.Errorf("ginjson.API 反序列化 = %d, 应认字符串形态", back.ID)
	}
}
