// Package excel 提供结构体 tag 驱动的通用 xlsx 导出。
//
// 在 VO 字段上打 `excel:"表头"` 即导出该列，列序即字段声明序；
// 需要值映射时再加 `excelDict:"0=正常,1=停用"`。没打 tag 的字段不导出。
//
//	type SysClientVo struct {
//	    ID     int64  `excel:"id"`
//	    Status string `excel:"状态" excelDict:"0=正常,1=停用"`
//	    Inner  []string // 无 tag，不导出
//	}
//
// 整个工作簿先在内存里建满再落笔，见 Export 的说明。
// 行数上限见 MaxRows：真要导出更大的量需改用 excelize 的 StreamWriter，
// 但它要求列宽在写行之前定好，与这里按内容自适应列宽的做法冲突。
package excel

import (
	"bytes"
	"fmt"
	"reflect"
	"unicode"

	"github.com/xuri/excelize/v2"

	"ruoyi-go-vue-plus/pkg/errs"
	"ruoyi-go-vue-plus/pkg/response"
)

// MaxRows 单次导出的数据行上限（不含表头）。
// 工作簿全程在内存里，没有上限时一次误操作就能把进程打爆。
const MaxRows = 100000

// headerNumFmt 内置的文本格式号，表头按文本存，避免 "1-2" 这类表头被识别成日期。
const headerNumFmt = 49

// 列宽边界：太窄看不见内容，太宽会把表撑得没法看。
const (
	minColWidth = 8.0
	maxColWidth = 60.0
	// widthPadding 给列宽留出的余量，字符宽度换算不精确，不留余量会看到 "###"。
	widthPadding = 2.0
)

// Options 导出可选项。
type Options struct {
	// SheetName 工作表名，同时用作下载文件名的主体。空则用 "sheet1"。
	SheetName string
	// MaxRows 行数上限，0 表示用 MaxRows。
	MaxRows int
}

// sheetName 返回工作表名。
func (o Options) sheetName() string {
	if o.SheetName == "" {
		return "sheet1"
	}
	return o.SheetName
}

// maxRows 返回行数上限。
func (o Options) maxRows() int {
	if o.MaxRows <= 0 {
		return MaxRows
	}
	return o.MaxRows
}

// Write 把 rows 写成 xlsx 字节流。
//
// rows 超过上限时返回业务异常而非截断：一份静默少了行的审计表比导出失败危险得多。
func Write[T any](rows []T, opts Options) (*bytes.Buffer, error) {
	cols, err := columnsOf[T]()
	if err != nil {
		return nil, err
	}

	if limit := opts.maxRows(); len(rows) > limit {
		return nil, errs.New(response.CodeFail,
			fmt.Sprintf("导出数据量过大，最多 %d 行，请缩小查询范围后重试", limit),
			fmt.Sprintf("excel: 待导出 %d 行超出上限 %d", len(rows), limit))
	}

	sheet := opts.sheetName()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	// NewFile 建的表叫 Sheet1，改名而非新建，免得留一张空表。
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, fmt.Errorf("excel: 工作表名 %q 无效: %w", sheet, err)
	}

	headers := make([]any, len(cols))
	for i, c := range cols {
		headers[i] = c.header
	}
	if err := f.SetSheetRow(sheet, "A1", &headers); err != nil {
		return nil, fmt.Errorf("excel: 写表头失败: %w", err)
	}

	// widths 记录每列的最宽内容，边写边量，省一趟遍历。
	widths := make([]float64, len(cols))
	for i, c := range cols {
		widths[i] = displayWidth(c.header)
	}

	cells := make([]any, len(cols))
	// next 是下一行的写入位置，与 rows 的下标脱钩：
	// 跳过 nil 元素时不能让行号跟着空掉，否则表里会留下空行。
	next := 2
	for _, row := range rows {
		rv := reflect.ValueOf(row)
		for rv.Kind() == reflect.Pointer && !rv.IsNil() {
			rv = rv.Elem()
		}
		// 切片里的 nil 元素跳过，写出去会是一整行 "<nil>"。
		if !rv.IsValid() || rv.Kind() == reflect.Pointer {
			continue
		}

		for j, c := range cols {
			v := c.cellValue(rv)
			cells[j] = v
			if w := displayWidth(fmt.Sprint(v)); w > widths[j] {
				widths[j] = w
			}
		}
		axis, err := excelize.CoordinatesToCellName(1, next)
		if err != nil {
			return nil, fmt.Errorf("excel: 计算单元格坐标失败: %w", err)
		}
		if err := f.SetSheetRow(sheet, axis, &cells); err != nil {
			return nil, fmt.Errorf("excel: 写第 %d 行失败: %w", next, err)
		}
		next++
	}

	if err := styleHeader(f, sheet, len(cols)); err != nil {
		return nil, err
	}
	if err := setWidths(f, sheet, widths); err != nil {
		return nil, err
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("excel: 生成工作簿失败: %w", err)
	}
	return buf, nil
}

// styleHeader 表头加粗并按文本格式存。
func styleHeader(f *excelize.File, sheet string, cols int) error {
	if cols == 0 {
		return nil
	}
	style, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		NumFmt:    headerNumFmt,
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return fmt.Errorf("excel: 创建表头样式失败: %w", err)
	}
	last, err := excelize.CoordinatesToCellName(cols, 1)
	if err != nil {
		return fmt.Errorf("excel: 计算表头范围失败: %w", err)
	}
	if err := f.SetCellStyle(sheet, "A1", last, style); err != nil {
		return fmt.Errorf("excel: 设置表头样式失败: %w", err)
	}
	return nil
}

// setWidths 按内容设置列宽。
func setWidths(f *excelize.File, sheet string, widths []float64) error {
	for i, w := range widths {
		name, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return fmt.Errorf("excel: 计算列名失败: %w", err)
		}
		if err := f.SetColWidth(sheet, name, name, clampWidth(w)); err != nil {
			return fmt.Errorf("excel: 设置列 %s 宽度失败: %w", name, err)
		}
	}
	return nil
}

// clampWidth 把测量宽度收进上下界。
func clampWidth(w float64) float64 {
	w += widthPadding
	if w < minColWidth {
		return minColWidth
	}
	if w > maxColWidth {
		return maxColWidth
	}
	return w
}

// displayWidth 估算字符串占的列宽，中日韩字符按双宽算。
func displayWidth(s string) float64 {
	var w float64
	for _, r := range s {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) ||
			unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			w += 2
			continue
		}
		w++
	}
	return w
}
