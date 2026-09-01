package excel

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

// testOrdered 字段故意乱序声明，验证列序跟声明序、不跟字母序。
type testOrdered struct {
	Zeta  string `excel:"Z"`
	Alpha string `excel:"A"`
	Mid   string `excel:"M"`
}

func TestColumnsOfOrder(t *testing.T) {
	cols, err := columnsOf[testOrdered]()
	if err != nil {
		t.Fatalf("columnsOf 失败: %v", err)
	}
	want := []string{"Z", "A", "M"}
	if len(cols) != len(want) {
		t.Fatalf("列数 = %d, 期望 %d", len(cols), len(want))
	}
	for i, w := range want {
		if cols[i].header != w {
			t.Errorf("第 %d 列 = %q, 期望 %q (列序应跟声明序)", i, cols[i].header, w)
		}
	}
}

func TestColumnsOfSkipsUntagged(t *testing.T) {
	cols, err := columnsOf[testRow]()
	if err != nil {
		t.Fatalf("columnsOf 失败: %v", err)
	}
	for _, c := range cols {
		if c.header == "Internal" {
			t.Error("没有 excel tag 的字段不应导出")
		}
	}
}

func TestColumnsOfSkipsUnexported(t *testing.T) {
	cols, err := columnsOf[testRow]()
	if err != nil {
		t.Fatalf("columnsOf 失败: %v", err)
	}
	for _, c := range cols {
		if c.header == "hidden" {
			t.Error("未导出字段不应出现在列里")
		}
	}
}

// TestColumnsOfFlattensEmbedded 匿名嵌入结构体的字段就地展开，位置在嵌入处。
func TestColumnsOfFlattensEmbedded(t *testing.T) {
	type base struct {
		CreateBy string `excel:"创建人"`
	}
	type row struct {
		ID int64 `excel:"id"`
		*base
		Name string `excel:"名称"`
	}

	cols, err := columnsOf[row]()
	if err != nil {
		t.Fatalf("columnsOf 失败: %v", err)
	}
	want := []string{"id", "创建人", "名称"}
	if len(cols) != len(want) {
		t.Fatalf("列数 = %d, 期望 %d (得到 %v)", len(cols), len(want), colHeaders(cols))
	}
	for i, w := range want {
		if cols[i].header != w {
			t.Errorf("第 %d 列 = %q, 期望 %q (嵌入字段应就地展开)", i, cols[i].header, w)
		}
	}
}

// TestColumnsOfCached 同一类型第二次解析走缓存，内容一致、不再报错。
func TestColumnsOfCached(t *testing.T) {
	first, err := columnsOf[testRow]()
	if err != nil {
		t.Fatalf("首次解析失败: %v", err)
	}
	second, err := columnsOf[testRow]()
	if err != nil {
		t.Fatalf("第二次解析失败: %v", err)
	}
	if len(first) != len(second) {
		t.Fatalf("两次列数不一致: %d vs %d", len(first), len(second))
	}
}

// TestColumnsOfCachedFailure 解析失败也进缓存，重试不该重复反射或抖动结果。
func TestColumnsOfCachedFailure(t *testing.T) {
	type bad struct {
		X string // 无 tag
	}
	if _, err := columnsOf[bad](); err == nil {
		t.Fatal("无 tag 类型应报错")
	}
	if _, err := columnsOf[bad](); err == nil {
		t.Fatal("缓存到失败结果后仍应报错")
	}
}

// TestColumnsOfNonStruct 非结构体类型直接报错，不 panic。
func TestColumnsOfNonStruct(t *testing.T) {
	if _, err := columnsOf[int](); err == nil {
		t.Error("int 类型应报错而非返回空列")
	}
	if _, err := columnsOf[[]string](); err == nil {
		t.Error("切片类型应报错")
	}
}

func colHeaders(cols []column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.header
	}
	return out
}

func TestParseDict(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    []dictPair
		wantErr bool
	}{
		{"空表达式", "", nil, false},
		{"两项", "0=正常,1=停用", []dictPair{{"0", "正常"}, {"1", "停用"}}, false},
		{"单项", "0=正常", []dictPair{{"0", "正常"}}, false},
		// 标签自己含 =，按首个 = 切分（对齐 Java split("=", 2)）。
		{"标签含等号", "0=a=b", []dictPair{{"0", "a=b"}}, false},
		{"缺等号", "正常", nil, true},
		{"某项缺等号", "0=正常,停用", nil, true},
		{"空片段跳过", "0=正常,,1=停用", []dictPair{{"0", "正常"}, {"1", "停用"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDict(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("表达式 %q 应报错", tt.expr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDict(%q) 失败: %v", tt.expr, err)
			}
			if !equalPairs(got, tt.want) {
				t.Errorf("parseDict(%q) = %v, 期望 %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestColumnLabel(t *testing.T) {
	c := column{dict: []dictPair{{"0", "正常"}, {"1", "停用"}}}
	tests := []struct {
		in   string
		want string
	}{
		{"0", "正常"},
		{"1", "停用"},
		// 未命中原样返回，不是留空——导出是给人看的，留空容易误判成该行没值。
		{"9", "9"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := c.label(tt.in); got != tt.want {
			t.Errorf("label(%q) = %q, 期望 %q", tt.in, got, tt.want)
		}
	}
}

func equalPairs(a, b []dictPair) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestWriteHeaderColumnType 表头按文本格式落格，避免 "1-2" 被 Excel 当成日期。
func TestWriteHeaderColumnType(t *testing.T) {
	type dateLike struct {
		Code string `excel:"1-2"`
	}
	buf, err := Write([]dateLike{{Code: "x"}}, Options{SheetName: "表头"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	val, err := f.GetCellValue("表头", "A1")
	if err != nil {
		t.Fatalf("取表头失败: %v", err)
	}
	if val != "1-2" {
		t.Errorf("表头值 = %q, 期望 %q (没被 Excel 当日期改写)", val, "1-2")
	}
}
