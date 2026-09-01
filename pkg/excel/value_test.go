package excel

import (
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// cellOf 导出单行后回读某个单元格的值与类型。
func cellOf(t *testing.T, row testRow, axis string) (string, excelize.CellType) {
	t.Helper()
	buf, err := Write([]testRow{row}, Options{SheetName: "值"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	val, err := f.GetCellValue("值", axis)
	if err != nil {
		t.Fatalf("取单元格 %s 失败: %v", axis, err)
	}
	typ, err := f.GetCellType("值", axis)
	if err != nil {
		t.Fatalf("取单元格 %s 类型失败: %v", axis, err)
	}
	return val, typ
}

// TestSnowflakeIDWrittenAsString 雪花 ID 必须以文本落格。
// 这是本包最关键的一条：写成数值 Excel 会显示 1.7611E+18 并把低位抹成 0，
// 导出的 id 拿回来对不上库里的行。
func TestSnowflakeIDWrittenAsString(t *testing.T) {
	const id = 1761100000000000001

	val, typ := cellOf(t, testRow{ID: id}, "A2")
	if val != "1761100000000000001" {
		t.Errorf("id 单元格值 = %q, 期望 %q (19 位不能丢精度)", val, "1761100000000000001")
	}
	// 数值单元格在 xlsx 里省略 t 属性，excelize 回读成 Unset；字符串才是 SharedString。
	if typ != excelize.CellTypeSharedString {
		t.Errorf("id 单元格类型 = %v, 期望 SharedString (不能是数值型)", typ)
	}
}

// TestSmallIntWrittenAsNumber 位数安全的整数保持数值型，仍可在 Excel 里参与计算。
func TestSmallIntWrittenAsNumber(t *testing.T) {
	val, typ := cellOf(t, testRow{ID: 123}, "A2")
	if val != "123" {
		t.Errorf("id 单元格值 = %q, 期望 %q", val, "123")
	}
	if typ != excelize.CellTypeUnset {
		t.Errorf("小整数单元格类型 = %v, 期望 Unset (数值型)", typ)
	}
}

func TestBigIntCell(t *testing.T) {
	tests := []struct {
		name     string
		digits   string
		isString bool
	}{
		{"15 位保持数值", "999999999999999", false},
		{"16 位转字符串", "1000000000000000", true},
		{"19 位雪花 ID 转字符串", "1761100000000000001", true},
		{"零", "0", false},
		// 负号计入长度，故 15 位负数也会转字符串。宁可多转：
		// 转过头只丢失可计算性，转不够则直接丢数据。
		{"15 位负数因负号转字符串", "-999999999999999", true},
		{"14 位负数保持数值", "-99999999999999", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bigIntCell(tt.digits, int64(1))
			_, isStr := got.(string)
			if isStr != tt.isString {
				t.Errorf("bigIntCell(%q) 返回字符串 = %v, 期望 %v", tt.digits, isStr, tt.isString)
			}
			if isStr && got.(string) != tt.digits {
				t.Errorf("bigIntCell(%q) = %q, 期望原样返回", tt.digits, got)
			}
		})
	}
}

// TestNormalizeNilPointerLeavesCellEmpty nil 指针必须留空。
// 少了解指针的 IsNil 判断，excelize 会走 default 分支把字面量 "<nil>" 写进表格。
func TestNormalizeNilPointerLeavesCellEmpty(t *testing.T) {
	tests := []struct {
		name string
		axis string
		row  testRow
	}{
		{"nil *string", "E2", testRow{Remark: nil}},
		{"nil *time.Time", "F2", testRow{LoginAt: nil}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := cellOf(t, tt.row, tt.axis)
			if val != "" {
				t.Errorf("单元格值 = %q, 期望空串(不能写成 <nil>)", val)
			}
		})
	}
}

func TestNormalizeNonNilPointer(t *testing.T) {
	remark := "备注内容"
	val, _ := cellOf(t, testRow{Remark: &remark}, "E2")
	if val != remark {
		t.Errorf("单元格值 = %q, 期望 %q", val, remark)
	}
}

func TestNormalizeTime(t *testing.T) {
	at := time.Date(2026, 9, 1, 14, 30, 5, 0, time.UTC)
	val, _ := cellOf(t, testRow{LoginAt: &at}, "F2")
	if val != "2026-09-01 14:30:05" {
		t.Errorf("时间单元格值 = %q, 期望 %q", val, "2026-09-01 14:30:05")
	}
}

func TestNormalizeZeroTimeLeavesCellEmpty(t *testing.T) {
	var zero time.Time
	val, _ := cellOf(t, testRow{LoginAt: &zero}, "F2")
	if val != "" {
		t.Errorf("零值时间单元格 = %q, 期望空串", val)
	}
}

// TestDictColumnRendersLabel 字典列导出的是标签而不是原始码值。
func TestDictColumnRendersLabel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"正常", "0", "正常"},
		{"停用", "1", "停用"},
		// 字典没覆盖的值原样输出，便于看出是数据脏了而不是这行没值。
		{"未命中原样输出", "9", "9"},
		{"空值仍为空", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, _ := cellOf(t, testRow{Status: tt.in}, "C2")
			if val != tt.want {
				t.Errorf("状态单元格 = %q, 期望 %q", val, tt.want)
			}
		})
	}
}

// TestNormalizeSliceLeavesCellEmpty 切片没有合理的单格形态，留空而不是打出 Go 语法。
func TestNormalizeSliceLeavesCellEmpty(t *testing.T) {
	type sliceRow struct {
		Items []string `excel:"条目"`
	}
	buf, err := Write([]sliceRow{{Items: []string{"a", "b"}}}, Options{SheetName: "切片"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	val, err := f.GetCellValue("切片", "A2")
	if err != nil {
		t.Fatalf("取单元格失败: %v", err)
	}
	if val != "" {
		t.Errorf("切片单元格 = %q, 期望空串", val)
	}
}

// TestFieldByIndexNilEmbeddedPointer 嵌入的 nil 指针不能让导出 panic。
// reflect.Value.FieldByIndex 在这种情况下会直接 panic，故本包自己走多级取值。
func TestFieldByIndexNilEmbeddedPointer(t *testing.T) {
	type base struct {
		CreateBy string `excel:"创建人"`
	}
	type row struct {
		*base
		Name string `excel:"名称"`
	}

	buf, err := Write([]row{{Name: "无嵌入值"}}, Options{SheetName: "嵌入"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := f.GetRows("嵌入")
	if err != nil {
		t.Fatalf("读行失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("行数 = %d, 期望 2", len(got))
	}
	if got[0][0] != "创建人" || got[0][1] != "名称" {
		t.Errorf("表头 = %v, 期望 [创建人 名称] (嵌入字段就地展开)", got[0])
	}
}

func TestFieldByIndexNonNilEmbeddedPointer(t *testing.T) {
	type base struct {
		CreateBy string `excel:"创建人"`
	}
	type row struct {
		*base
		Name string `excel:"名称"`
	}

	buf, err := Write([]row{{base: &base{CreateBy: "admin"}, Name: "有嵌入值"}},
		Options{SheetName: "嵌入2"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	val, err := f.GetCellValue("嵌入2", "A2")
	if err != nil {
		t.Fatalf("取单元格失败: %v", err)
	}
	if val != "admin" {
		t.Errorf("嵌入字段值 = %q, 期望 %q", val, "admin")
	}
}
