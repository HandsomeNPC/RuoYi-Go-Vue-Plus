package treeutil

import (
	"cmp"
	"slices"
)

// ParseFunc 业务对象 → 树节点的字段搬运回调。
//
// 框架遍历列表时为每个 item 新建一个空 Tree 节点，再调 ParseFunc 让调用方把
// item 的字段搬进节点。框架只认 Tree 的 ID/ParentID/Name/Weight，不知道业务
// 对象长什么样（部门叫 DeptID、菜单叫 MenuID……），所以把映射权交给调用方：
// 调用方在此函数里至少填 node.ID 与 node.ParentID（挂接靠它们），Name/Weight/
// Extra 按需。
//
// 类型参数：T 业务对象类型（任意），K 节点 ID 类型（须可比较）。
// 入参：item 列表里的一个业务对象，node 框架已 new 好的空节点。
// 无返回值，就地改 node。
type ParseFunc[T any, K comparable] func(item T, node *Tree[K])

// BuildTree 构建单根树。
// 类型参数：T 业务对象类型，K 节点 ID 类型（须可比较）。
// 入参：list 业务对象列表，rootID 根父 ID，parse 把单个业务对象填进新建的树节点（至少设 ID/ParentID）。
// 出参：rootID 下的顶层节点列表，同层按 Weight 升序。
func BuildTree[T any, K comparable](list []T, rootID K, parse ParseFunc[T, K]) []*Tree[K] {
	nodes := make([]*Tree[K], 0, len(list))
	for _, item := range list {
		node := new(Tree[K])
		parse(item, node)
		nodes = append(nodes, node)
	}
	return assemble(nodes, []K{rootID})
}

// BuildMultiRootTree 构建多根树：parentID 不在所有 ID 集合内的节点即悬空根（祖先被过滤掉的场景），各自成根挂接。
// 类型参数：T 业务对象类型，K 节点 ID 类型（须可比较）。
// 入参：list 业务对象列表，parse 把单个业务对象填进新建的树节点（至少设 ID/ParentID）。
// 出参：顶层节点列表，多根按入参首次出现序、同层按 Weight 升序。
func BuildMultiRootTree[T any, K comparable](list []T, parse ParseFunc[T, K]) []*Tree[K] {
	nodes := make([]*Tree[K], 0, len(list))
	for _, item := range list {
		node := new(Tree[K])
		parse(item, node)
		nodes = append(nodes, node)
	}

	// 悬空根：节点的 parentID 不在任何节点的 ID 集合里，则该 parentID 是一个根父。
	// 按 nodes 顺序去重收集，保证多根顺序稳定。
	ids := make(map[K]struct{}, len(nodes))
	for _, n := range nodes {
		ids[n.ID] = struct{}{}
	}
	rootIDs := make([]K, 0)
	seen := make(map[K]struct{})
	for _, n := range nodes {
		if _, isInternal := ids[n.ParentID]; isInternal {
			continue
		}
		if _, dup := seen[n.ParentID]; dup {
			continue
		}
		seen[n.ParentID] = struct{}{}
		rootIDs = append(rootIDs, n.ParentID)
	}

	return assemble(nodes, rootIDs)
}

// assemble 把一堆散的树节点按父子关系挂接成树，返回指定的顶层根节点。
// BuildTree 和 BuildMultiRootTree 共用它。
// 类型参数：K 节点 ID 类型（须可比较），用于做 ParentID→ID 的 map 键。
//
// 入参：
//   - nodes：已经填好 ID/ParentID 的节点，但 Children 还没挂接。
//   - rootIDs：哪些 ParentID 算作顶层根。单根树传 []K{0}；多根树传多个悬空父 ID。
//
// 出参：挂在 rootIDs 下的顶层节点列表，同一父节点下的兄弟按 Weight 升序排好。
//
// 只做挂接、不往下递归找父节点：如果某节点的父节点不在 nodes 里（比如被条件过滤掉了），
// 它既不会被挂到任何节点下，也不会冒到返回结果里——直接丢弃。
func assemble[K comparable](nodes []*Tree[K], rootIDs []K) []*Tree[K] {
	childrenOf := make(map[K][]*Tree[K], len(nodes))
	for _, n := range nodes {
		childrenOf[n.ParentID] = append(childrenOf[n.ParentID], n)
	}
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
