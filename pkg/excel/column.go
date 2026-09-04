package excel

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
)

const (
	// tagHeader 表头文案，字段没有这个 tag 即不导出。
	tagHeader = "excel"
	// tagDict 值映射表达式，形如 "0=正常,1=停用"。
	tagDict = "excelDict"
)

// column 一个导出列。
type column struct {
	header string
	// index 走 FieldByIndex 的多级下标，匿名嵌入的字段会有多级。
	index []int
	dict  []dictPair
}

// dictPair 一条值映射。用切片而非 map：表达式里的顺序是人写的，
// 保留顺序便于出错时按原样回报，且字典通常只有两三项，线性查找更快。
type dictPair struct {
	value string
	label string
}

// columnsResult 缓存一个类型的解析结果，解析失败也缓存，避免每次导出重复走反射。
type columnsResult struct {
	columns []column
	err     error
}

var columnCache sync.Map // reflect.Type -> columnsResult

// columnsOf 解析 T 的导出列，按字段声明序返回。
func columnsOf[T any]() ([]column, error) {
	var zero T
	t := reflect.TypeOf(&zero).Elem()
	// 支持 []*Vo：元素是指针时取指向的结构体。
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("excel: 导出类型须是结构体，得到 %s", t.Kind())
	}

	if cached, ok := columnCache.Load(t); ok {
		res := cached.(columnsResult)
		return res.columns, res.err
	}

	cols, err := parseColumns(t, nil)
	if err == nil && len(cols) == 0 {
		err = fmt.Errorf("excel: 类型 %s 没有任何带 %q tag 的字段", t.Name(), tagHeader)
	}
	columnCache.Store(t, columnsResult{columns: cols, err: err})
	return cols, err
}

// parseColumns 递归收集带 tag 的字段，匿名嵌入结构体的字段就地展开。
func parseColumns(t reflect.Type, prefix []int) ([]column, error) {
	var cols []column
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		index := append(append([]int(nil), prefix...), i)

		if f.Anonymous {
			et := f.Type
			for et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			if et.Kind() == reflect.Struct {
				sub, err := parseColumns(et, index)
				if err != nil {
					return nil, err
				}
				cols = append(cols, sub...)
				continue
			}
		}

		header, ok := f.Tag.Lookup(tagHeader)
		if !ok || header == "" {
			continue
		}
		// 未导出字段反射取不到值，与其导出一整列空白不如直接报错。
		if f.PkgPath != "" {
			return nil, fmt.Errorf("excel: 字段 %s.%s 未导出，不能带 %q tag", t.Name(), f.Name, tagHeader)
		}

		dict, err := parseDict(f.Tag.Get(tagDict))
		if err != nil {
			return nil, fmt.Errorf("excel: 字段 %s.%s 的 %q tag 无效: %w", t.Name(), f.Name, tagDict, err)
		}
		cols = append(cols, column{header: header, index: index, dict: dict})
	}
	return cols, nil
}

// parseDict 解析值映射表达式 "0=正常,1=停用"。
func parseDict(expr string) ([]dictPair, error) {
	if expr == "" {
		return nil, nil
	}
	var pairs []dictPair
	for _, seg := range strings.Split(expr, ",") {
		if seg = strings.TrimSpace(seg); seg == "" {
			continue
		}
		// 按首个 = 切分，标签自身可以含 =。
		v, label, ok := strings.Cut(seg, "=")
		if !ok {
			return nil, fmt.Errorf("片段 %q 缺少 =", seg)
		}
		pairs = append(pairs, dictPair{value: v, label: label})
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("表达式 %q 没有有效片段", expr)
	}
	return pairs, nil
}

// label 把原始值换成字典标签。
// 未命中时原样返回而非留空：
// 导出是给人做审计的，留空会让"字典缺了一项"和"这行本来就没值"看起来一样。
func (c column) label(raw string) string {
	if raw == "" || len(c.dict) == 0 {
		return raw
	}
	for _, p := range c.dict {
		if p.value == raw {
			return p.label
		}
	}
	return raw
}
