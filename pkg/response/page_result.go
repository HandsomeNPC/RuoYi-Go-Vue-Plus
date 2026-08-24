package response

// PageResult 表格分页数据对象，不含 code/msg，需嵌在 R.data 里返回。
type PageResult[T any] struct {
	Total int64 `json:"total"` // 总记录数
	Rows  []T   `json:"rows"`  // 列表数据
}

// Page 根据列表和总数构建分页结果。
func Page[T any](rows []T, total int64) PageResult[T] {
	return PageResult[T]{Total: total, Rows: emptyIfNil(rows)}
}

// PageOf 根据列表构建分页结果，总数取列表长度。
func PageOf[T any](rows []T) PageResult[T] {
	rows = emptyIfNil(rows)
	return PageResult[T]{Total: int64(len(rows)), Rows: rows}
}

// EmptyPage 构建空分页结果。
func EmptyPage[T any]() PageResult[T] {
	return PageResult[T]{Total: 0, Rows: []T{}}
}

// emptyIfNil 空切片兜底，保证序列化为 [] 而不是 null。
func emptyIfNil[T any](rows []T) []T {
	if rows == nil {
		return []T{}
	}
	return rows
}
