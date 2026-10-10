// Package treeutil 提供树形结构构建工具。
package treeutil

import (
	"bytes"
	"slices"

	"github.com/HandsomeNPC/ruoyi-common-go/jsonx"
)

// Tree 树节点。固定字段由结构体承载，Extra 承动态扩展键，序列化时拍平成单层对象。
// 类型参数：K 节点 ID 的类型，须可比较（如 int64、string）——挂接靠 ParentID→ID 比对，要做 map 键。
type Tree[K comparable] struct {
	// ID 节点自身 ID，序列化为 "id"。雪花值由全局 codec 转字符串。
	ID K
	// ParentID 父节点 ID，序列化为 "parentId"。根节点通常为 0。
	ParentID K
	// Name 显示名称，序列化为 "label"。
	Name string
	// Weight 排序权重，序列化为 "weight"，同层按升序排列。
	Weight int
	// Extra 动态扩展键，序列化时平铺到与 id/label 同层。
	Extra map[string]any
	// Children 子节点列表，序列化为 "children"；为空时省略该键（前端据此判叶子）。
	Children []*Tree[K]
}

// SetExtra 追加一个扩展键。入参：key 键名、val 值。返回节点自身以便链式调用。
func (t *Tree[K]) SetExtra(key string, val any) *Tree[K] {
	if t.Extra == nil {
		t.Extra = make(map[string]any)
	}
	t.Extra[key] = val
	return t
}

// MarshalJSON 将固定字段与 Extra 拍平成单层 JSON 对象。
// children 为空时省略该键——前端靠键的有无判断叶子节点。
func (t *Tree[K]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(128)
	buf.WriteByte('{')

	// 固定字段键名是无须转义的标识符，直接拼字面量；值走 jsonx（雪花 id 转字符串）。
	appendKV := func(key string, val any) error {
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('"')
		buf.WriteString(key)
		buf.WriteByte('"')
		buf.WriteByte(':')
		b, err := jsonx.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}

	if err := appendKV("id", t.ID); err != nil {
		return nil, err
	}
	if err := appendKV("parentId", t.ParentID); err != nil {
		return nil, err
	}
	if err := appendKV("label", t.Name); err != nil {
		return nil, err
	}
	if err := appendKV("weight", t.Weight); err != nil {
		return nil, err
	}

	// Extra 键名可能含特殊字符，键值都走 jsonx；按键名字典序输出保证稳定。
	if len(t.Extra) > 0 {
		keys := make([]string, 0, len(t.Extra))
		for k := range t.Extra {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			kb, err := jsonx.Marshal(k)
			if err != nil {
				return nil, err
			}
			vb, err := jsonx.Marshal(t.Extra[k])
			if err != nil {
				return nil, err
			}
			if buf.Len() > 1 {
				buf.WriteByte(',')
			}
			buf.Write(kb)
			buf.WriteByte(':')
			buf.Write(vb)
		}
	}

	if len(t.Children) > 0 {
		if err := appendKV("children", t.Children); err != nil {
			return nil, err
		}
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}
