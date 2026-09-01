package excel

import (
	"errors"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// testRow 覆盖各类字段：带 tag/不带 tag、字典列、雪花 ID、指针、时间。
type testRow struct {
	ID       int64      `excel:"id"`
	Name     string     `excel:"名称"`
	Status   string     `excel:"状态" excelDict:"0=正常,1=停用"`
	Count    int        `excel:"数量"`
	Remark   *string    `excel:"备注"`
	LoginAt  *time.Time `excel:"登录时间"`
	Internal []string   // 无 tag，不该导出
	hidden   string     // 未导出字段，无 tag，应被忽略
}

// openRows 把导出结果读回来，返回全部行。
func openRows(t *testing.T, rows []testRow, opts Options) [][]string {
	t.Helper()
	buf, err := Write(rows, opts)
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读工作簿失败: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	sheet := opts.sheetName()
	got, err := f.GetRows(sheet)
	if err != nil {
		t.Fatalf("读取工作表 %q 失败: %v", sheet, err)
	}
	return got
}

func TestWriteHeaderRow(t *testing.T) {
	got := openRows(t, []testRow{{}}, Options{SheetName: "测试"})
	want := []string{"id", "名称", "状态", "数量", "备注", "登录时间"}

	if len(got) == 0 {
		t.Fatal("没有写出任何行")
	}
	if len(got[0]) != len(want) {
		t.Fatalf("表头列数 = %d, 期望 %d (得到 %v)", len(got[0]), len(want), got[0])
	}
	for i, w := range want {
		if got[0][i] != w {
			t.Errorf("第 %d 列表头 = %q, 期望 %q", i, got[0][i], w)
		}
	}
}

func TestWriteSheetName(t *testing.T) {
	buf, err := Write([]testRow{}, Options{SheetName: "客户端管理"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) != 1 {
		t.Fatalf("工作表数 = %d, 期望 1 (得到 %v)", len(sheets), sheets)
	}
	if sheets[0] != "客户端管理" {
		t.Errorf("工作表名 = %q, 期望 %q", sheets[0], "客户端管理")
	}
}

func TestWriteSheetNameDefault(t *testing.T) {
	buf, err := Write([]testRow{}, Options{})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	if got := f.GetSheetList(); len(got) != 1 || got[0] != "sheet1" {
		t.Errorf("默认工作表名 = %v, 期望 [sheet1]", got)
	}
}

func TestWriteEmptyRows(t *testing.T) {
	tests := []struct {
		name string
		rows []testRow
	}{
		{"nil 切片", nil},
		{"空切片", []testRow{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := openRows(t, tt.rows, Options{SheetName: "空"})
			if len(got) != 1 {
				t.Errorf("行数 = %d, 期望 1 (只有表头)", len(got))
			}
		})
	}
}

func TestWriteRejectsTooManyRows(t *testing.T) {
	rows := []testRow{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	_, err := Write(rows, Options{SheetName: "超限", MaxRows: 2})
	if err == nil {
		t.Fatal("超出行数上限应报错")
	}

	// 必须是业务异常，否则 Recover 只会回一句"发生未知异常"，用户不知道该缩小范围。
	var se *errs.ServiceError
	if !errors.As(err, &se) {
		t.Fatalf("错误类型 = %T, 期望 *errs.ServiceError", err)
	}
	if se.Code != response.CodeFail {
		t.Errorf("Code = %d, 期望 %d", se.Code, response.CodeFail)
	}
	if se.Msg == "" {
		t.Error("Msg 不该为空，前端要靠它提示用户")
	}
}

func TestWriteAllowsExactlyMaxRows(t *testing.T) {
	rows := []testRow{{Name: "a"}, {Name: "b"}}
	if _, err := Write(rows, Options{SheetName: "边界", MaxRows: 2}); err != nil {
		t.Errorf("正好等于上限不该报错, 得到 %v", err)
	}
}

func TestWriteRejectsNonStruct(t *testing.T) {
	if _, err := Write([]int{1, 2}, Options{}); err == nil {
		t.Error("非结构体元素应报错而非 panic")
	}
	if _, err := Write([]string{"a"}, Options{}); err == nil {
		t.Error("非结构体元素应报错而非 panic")
	}
}

func TestWriteRejectsUntaggedStruct(t *testing.T) {
	type untagged struct {
		Name string
	}
	if _, err := Write([]untagged{{Name: "a"}}, Options{}); err == nil {
		t.Error("没有任何 excel tag 的类型应报错")
	}
}

func TestWriteInvalidSheetName(t *testing.T) {
	tests := []struct {
		name  string
		sheet string
	}{
		{"含非法字符", "a:b"},
		{"超长", "这个名字实在是太长了超过三十一个字符的限制了啊啊啊啊啊啊啊啊啊啊啊啊啊啊"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Write([]testRow{}, Options{SheetName: tt.sheet}); err == nil {
				t.Errorf("非法工作表名 %q 应报错", tt.sheet)
			}
		})
	}
}

func TestWriteSkipsNilPointerElements(t *testing.T) {
	v := testRow{Name: "有效"}
	buf, err := Write([]*testRow{nil, &v, nil}, Options{SheetName: "跳过"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	got, err := f.GetRows("跳过")
	if err != nil {
		t.Fatalf("读行失败: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("行数 = %d, 期望 2 (表头 + 1 条有效数据)", len(got))
	}
	if got[1][1] != "有效" {
		t.Errorf("数据行名称 = %q, 期望 %q", got[1][1], "有效")
	}
}

func TestWriteHeaderStyle(t *testing.T) {
	buf, err := Write([]testRow{}, Options{SheetName: "样式"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	id, err := f.GetCellStyle("样式", "A1")
	if err != nil {
		t.Fatalf("取表头样式失败: %v", err)
	}
	style, err := f.GetStyle(id)
	if err != nil {
		t.Fatalf("解析表头样式失败: %v", err)
	}
	if style.Font == nil || !style.Font.Bold {
		t.Error("表头应加粗")
	}
	if style.NumFmt != headerNumFmt {
		t.Errorf("表头 NumFmt = %d, 期望 %d (文本格式)", style.NumFmt, headerNumFmt)
	}
}

func TestWriteColumnWidthClamped(t *testing.T) {
	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	buf, err := Write([]testRow{{Name: long}}, Options{SheetName: "列宽"})
	if err != nil {
		t.Fatalf("Write 失败: %v", err)
	}
	f, err := excelize.OpenReader(buf)
	if err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	defer func() { _ = f.Close() }()

	// A 列表头 "id" 只有 2 字符，应被抬到下界。
	gotA, err := f.GetColWidth("列宽", "A")
	if err != nil {
		t.Fatalf("取 A 列宽失败: %v", err)
	}
	if gotA != minColWidth {
		t.Errorf("A 列宽 = %v, 期望 %v (下界)", gotA, minColWidth)
	}

	// B 列有 200 字符内容，应被压到上界。
	gotB, err := f.GetColWidth("列宽", "B")
	if err != nil {
		t.Fatalf("取 B 列宽失败: %v", err)
	}
	if gotB != maxColWidth {
		t.Errorf("B 列宽 = %v, 期望 %v (上界)", gotB, maxColWidth)
	}
}

func TestClampWidth(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"低于下界", 1, minColWidth},
		{"正好下界", minColWidth - widthPadding, minColWidth},
		{"区间内", 20, 20 + widthPadding},
		{"超出上界", 500, maxColWidth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampWidth(tt.in); got != tt.want {
				t.Errorf("clampWidth(%v) = %v, 期望 %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{"空串", "", 0},
		{"纯 ASCII", "abc", 3},
		{"纯中文按双宽", "客户端", 6},
		{"中英混排", "id客户端", 8},
		{"日文按双宽", "ひらがな", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayWidth(tt.in); got != tt.want {
				t.Errorf("displayWidth(%q) = %v, 期望 %v", tt.in, got, tt.want)
			}
		})
	}
}
