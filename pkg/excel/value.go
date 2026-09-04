package excel

import (
	"fmt"
	"reflect"
	"strconv"
	"time"
)

// maxSafeDigits 数字保持数值型的最大十进制位数。
// 超过这个位数 Excel 会按浮点存、把尾数抹成 0（雪花 ID 是 19 位，
// 直接写数值会显示成 1.7611E+18 且低位丢失），故转字符串。
// 15 位阈值，保证大数渲染。
const maxSafeDigits = 15

// timeLayout 时间列的渲染格式，对齐项目其它出参。
const timeLayout = "2006-01-02 15:04:05"

// cellValue 取一列在某行上的单元格值，返回可直接交给 excelize 的类型。
func (c column) cellValue(row reflect.Value) any {
	f, ok := fieldByIndex(row, c.index)
	if !ok {
		return nil
	}

	v := normalize(f)
	// 带字典的列一律按字符串比对：tag 里写的是文本，字段却可能是数字类型。
	if len(c.dict) > 0 {
		if v == nil {
			return ""
		}
		return c.label(fmt.Sprint(v))
	}
	return v
}

// normalize 把反射值转成 excelize 认得的标量。
func normalize(v reflect.Value) any {
	// 先解指针：excelize 的 SetCellValue 落到 default 分支会把 *T 打成 "0xc000…"，
	// 而 nil 指针会被 fmt.Sprint 写成字面量 "<nil>"。
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if t, ok := v.Interface().(time.Time); ok {
		if t.IsZero() {
			return nil
		}
		return t.Format(timeLayout)
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return bigIntCell(strconv.FormatInt(v.Int(), 10), v.Interface())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return bigIntCell(strconv.FormatUint(v.Uint(), 10), v.Interface())
	case reflect.Slice, reflect.Array, reflect.Map:
		// 这些类型没有合理的单格形态，留空而不是打出 Go 语法。
		// 需要导出集合的场景应在 VO 上另开一个已拼好的字符串字段。
		return nil
	default:
		return v.Interface()
	}
}

// bigIntCell 位数安全就保持数值型（Excel 里可参与计算、右对齐），否则转字符串保精度。
// 负号计入位数，故 -999999999999999 也会转字符串——宁可多转：
// 转过头只是失去计算能力，转不够则直接丢数据。
func bigIntCell(digits string, val any) any {
	if len(digits) > maxSafeDigits {
		return digits
	}
	return val
}

// fieldByIndex 按多级下标取字段，遇到 nil 的嵌入指针返回 false。
// 不用 reflect.Value.FieldByIndex：它在这种情况下直接 panic。
func fieldByIndex(v reflect.Value, index []int) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 {
			for v.Kind() == reflect.Pointer {
				if v.IsNil() {
					return reflect.Value{}, false
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v, true
}
