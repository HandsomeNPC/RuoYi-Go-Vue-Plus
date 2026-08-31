// Package tree 树形结构构建工具，对应 Java org.dromara.common.core.utils.TreeBuildUtils。
package tree

import (
	"bytes"
	"cmp"
	"encoding/json"
	"slices"
)

// Tree 树节点，对应 hutool 的 Tree<T>。
//
// hutool 的 Tree 是 LinkedHashMap<String,Object> 子类，固定键与 putExtra 的动态键
// 平级共存，序列化出的是扁平 JSON。Go 这边用结构体承固定字段、Extra 承动态键，
// 靠 MarshalJSON 把两者拍平，以复刻同样的线上格式（前端 MenuTreeOption /
// DeptTreeVO 就是按这个扁平形状声明的）。
type Tree[K comparable] struct {
	ID       K
	ParentID K
	// Name 序列化为 "label"，对应 RuoYi 定制的 TreeNodeConfig.setNameKey("label")。
	Name   string
	Weight int
	// Extra 动态扩展键，对应 hutool putExtra；序列化时平铺到与 id/label 同层。
	Extra    map[string]any
	Children []*Tree[K]
}

// SetExtra 追加扩展键，对应 hutool Tree.putExtra。
func (t *Tree[K]) SetExtra(key string, val any) *Tree[K] {
	if t.Extra == nil {
		t.Extra = make(map[string]any)
	}
	t.Extra[key] = val
	return t
}

// MarshalJSON 把固定字段与 Extra 拍平成单层对象。
//
// 两处有意偏离 hutool：
//   - Extra 按键名字典序输出，而非 putExtra 的插入序。Go map 无序，不排序则同一份数据
//     每次序列化键序都在漂移，golden 测试和响应 diff 无从做起。JSON 对象键序无语义。
//   - weight 恒输出。hutool 在 weight 为 null 时省略该键，但 RuoYi 所有调用点都传
//     orderNum，前端也声明为必填 number，不存在缺失场景。
//
// children 为空时省略该键这条**必须**照搬 hutool：前端靠键的有无判断叶子节点。
func (t *Tree[K]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	writeKV := func(key string, val any) error {
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(key)
		if err != nil {
			return err
		}
		buf.Write(k)
		buf.WriteByte(':')
		v, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(v)
		return nil
	}

	for _, kv := range []struct {
		key string
		val any
	}{
		{"id", t.ID},
		{"parentId", t.ParentID},
		{"label", t.Name},
		{"weight", t.Weight},
	} {
		if err := writeKV(kv.key, kv.val); err != nil {
			return nil, err
		}
	}

	keys := make([]string, 0, len(t.Extra))
	for k := range t.Extra {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if err := writeKV(k, t.Extra[k]); err != nil {
			return nil, err
		}
	}

	if len(t.Children) > 0 {
		if err := writeKV("children", t.Children); err != nil {
			return nil, err
		}
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Build 构建单根树，对应 Java TreeBuildUtils.build(list, parentId, nodeParser)。
// parse 负责把业务对象填进树节点（至少要设置 ID/ParentID）。同层按 Weight 稳定升序。
func Build[T any, K comparable](list []T, rootID K, parse func(item T, node *Tree[K])) []*Tree[K] {
	nodes := make([]*Tree[K], 0, len(list))
	for _, item := range list {
		node := new(Tree[K])
		parse(item, node)
		nodes = append(nodes, node)
	}
	return assemble(nodes, []K{rootID})
}

// BuildMultiRoot 构建多根树，对应 Java TreeBuildUtils.buildMultiRoot。
// 根节点识别：所有 parentID 减去所有 ID，剩下的即为悬空父级（如按条件过滤后祖先不在结果集内）。
func BuildMultiRoot[T any, K comparable](list []T, getID, getParentID func(T) K,
	parse func(item T, node *Tree[K])) []*Tree[K] {

	ids := make(map[K]struct{}, len(list))
	for _, item := range list {
		ids[getID(item)] = struct{}{}
	}

	// 按 list 中首次出现顺序收集根 parentID：range map 顺序随机，会让多根结果每次调用都在抖。
	rootIDs := make([]K, 0)
	seen := make(map[K]struct{})
	for _, item := range list {
		pid := getParentID(item)
		if _, isInternal := ids[pid]; isInternal {
			continue
		}
		if _, dup := seen[pid]; dup {
			continue
		}
		seen[pid] = struct{}{}
		rootIDs = append(rootIDs, pid)
	}

	nodes := make([]*Tree[K], 0, len(list))
	for _, item := range list {
		node := new(Tree[K])
		parse(item, node)
		nodes = append(nodes, node)
	}
	return assemble(nodes, rootIDs)
}

// BuildInPlace 就地构树：树节点就是原对象本身，通过 setChildren 回调回填子节点，
// 对应 Java TreeBuildUtils.build(items, parentId, classifier, action)。
//
// 与 Build/BuildMultiRoot 不同，本函数**不排序** —— Java 该重载只做 groupingBy 分组挂接，
// 顺序完全依赖 SQL 的 ORDER BY parent_id, order_num。调用方须自行保证入参已排好序。
func BuildInPlace[T any, K comparable](items []T, rootID K, getID, getParentID func(T) K,
	setChildren func(item T, children []T)) []T {

	childrenOf := make(map[K][]T, len(items))
	for _, item := range items {
		pid := getParentID(item)
		childrenOf[pid] = append(childrenOf[pid], item)
	}
	for _, item := range items {
		setChildren(item, childrenOf[getID(item)])
	}

	roots := childrenOf[rootID]
	if roots == nil {
		return []T{}
	}
	return roots
}

// assemble 按 ParentID 分组挂接并逐层排序，返回 rootIDs 下的顶层节点。
//
// 只做分组挂接、不递归下钻（与 Java 一致）：父节点不在 nodes 里的子树不会冒到顶层。
func assemble[K comparable](nodes []*Tree[K], rootIDs []K) []*Tree[K] {
	childrenOf := make(map[K][]*Tree[K], len(nodes))
	for _, n := range nodes {
		childrenOf[n.ParentID] = append(childrenOf[n.ParentID], n)
	}
	// SliceStable：weight 相同的节点保持插入序，对齐 hutool 基于 LinkedHashMap 的稳定排序。
	for _, siblings := range childrenOf {
		slices.SortStableFunc(siblings, func(a, b *Tree[K]) int {
			return cmp.Compare(a.Weight, b.Weight)
		})
	}
	for _, n := range nodes {
		n.Children = childrenOf[n.ID]
	}

	roots := make([]*Tree[K], 0)
	for _, rootID := range rootIDs {
		roots = append(roots, childrenOf[rootID]...)
	}
	return roots
}
