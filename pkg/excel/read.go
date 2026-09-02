package excel

import (
	"fmt"
	"io"
	"reflect"
	"strconv"
	"time"

	"github.com/xuri/excelize/v2"
)

// Read 把 r 里的 xlsx 读成 []T。
//
// 列定位按表头文本匹配：首行单元格文本与字段 excel tag 的值相等即该列，
// 与列顺序无关——模板若被人工调整列序仍能正确读回。
// 带字典的列（excelDict）做反向映射：单元格写的是标签（如"男"）时换回原值（"0"），
// 直接写原值（"0"）时原样保留，两种填法都能吃。
//
// 每个数据行构造一个 T，按列下标 FieldByIndex 赋值；空串数值列写零值。
// 空表头行或无数据行返回空切片，不算错。
func Read[T any](r io.Reader) ([]T, error) {
	var zero T
	t := reflect.TypeOf(&zero).Elem()
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("excel: 导入类型须是结构体，得到 %s", t.Kind())
	}

	cols, err := columnsOf[T]()
	if err != nil {
		return nil, err
	}

	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("excel: 打开导入文件失败: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("excel: 读取工作表 %q 失败: %w", sheet, err)
	}
	if len(rows) == 0 {
		return []T{}, nil
	}

	// 首行表头：文本 → 列号。重名列取最后一个出现位置，与人工直觉一致。
	header := rows[0]
	idx := make(map[string]int, len(header))
	for i, h := range header {
		idx[h] = i
	}

	out := make([]T, 0, len(rows)-1)
	for r := 1; r < len(rows); r++ {
		row := rows[r]
		// 全空行跳过：excelize 对尾部空行有时会回一个全空切片，写出去会是一整行零值。
		if !rowHasContent(row) {
			continue
		}

		v := reflect.New(t).Elem()
		for _, c := range cols {
			col, ok := idx[c.header]
			if !ok {
				// 模板里缺这列表头，字段留零。
				continue
			}
			if col >= len(row) {
				continue
			}
			if err := setCell(v, c, row[col]); err != nil {
				return nil, fmt.Errorf("excel: 第 %d 行 %q 列 %q 解析失败: %w",
					r+1, c.header, row[col], err)
			}
		}
		out = append(out, v.Interface().(T))
	}
	return out, nil
}

// rowHasContent 判断一行是否至少有一个非空单元格，过滤 excelize 回吐的尾部空行。
func rowHasContent(row []string) bool {
	for _, c := range row {
		if c != "" {
			return true
		}
	}
	return false
}

// setCell 把一个单元格文本写回结构体字段。
//
// 带字典的列先做反向映射（标签→原值），未命中保留原值；
// 之后按字段 Kind 分派：字符串直接写，整数解析（空串写零），time.Time 按出参同款格式解析。
func setCell(rv reflect.Value, c column, raw string) error {
	f, ok := fieldByIndex(rv, c.index)
	if !ok {
		return nil
	}
	// 解指针后才能 Set：字段可能是 *T 或匿名嵌入指针。
	for f.Kind() == reflect.Pointer {
		if f.IsNil() {
			// 导入字段一般不涉及指针，nil 时无法就地建值，跳过。
			return nil
		}
		f = f.Elem()
	}
	if !f.CanSet() {
		return nil
	}

	if raw == "" {
		return nil
	}
	// 字典反向：单元格写标签则换回原值。
	if len(c.dict) > 0 {
		for _, p := range c.dict {
			if p.label == raw {
				raw = p.value
				break
			}
		}
	}

	switch f.Kind() {
	case reflect.String:
		f.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return err
		}
		f.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return err
		}
		f.SetUint(n)
	case reflect.Bool:
		// 布尔列模板里通常写"是/否"或 true/false，只认真值。
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		f.SetBool(b)
	default:
		// 兜底尝试 time.Time：导入场景少见，按出参同款格式解析。
		if _, ok := f.Interface().(time.Time); ok {
			if tm, err := time.ParseInLocation(timeLayout, raw, time.Local); err == nil {
				f.Set(reflect.ValueOf(tm))
			}
		}
	}
	return nil
}
