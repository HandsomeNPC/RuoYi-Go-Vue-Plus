package treeutil

// AssembleTree 把一组业务对象就地拼装成树：对象本身即节点，通过 setChildren 回填子节点。不排序，顺序依赖入参。
// 类型参数：T 业务对象类型（同时充当树节点），K 节点 ID 类型（须可比较）。
// 入参：items 业务对象列表，rootID 根父 ID，getID/getParentID 取 ID 与父 ID，setChildren 把子节点回填到对象上。
// 出参：rootID 下的顶层对象列表。
func AssembleTree[T any, K comparable](items []T, rootID K, getID, getParentID func(T) K,
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
