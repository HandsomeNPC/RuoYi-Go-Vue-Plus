package response

// PageResult 表格分页数据对象，对应原项目 domain.PageResult<T>。
//
// 它只承载数据本身，不含 code/msg —— 原项目 Controller 返回的是
// R<PageResult<T>>，即分页结果嵌在 R.data 里：
//
//	{"code":200,"msg":"操作成功","data":{"total":10,"rows":[...]}}
//
// 所以 handler 里要写 response.Ok(response.Page(rows, total))，
// 不要把 PageResult 直接当顶层响应体返回。
type PageResult[T any] struct {
	Total int64 `json:"total"` // 总记录数
	Rows  []T   `json:"rows"`  // 列表数据
}

// Page 根据列表和总数构建分页结果，对应 PageResult.build(list, total)。
func Page[T any](rows []T, total int64) PageResult[T] {
	return PageResult[T]{Total: total, Rows: emptyIfNil(rows)}
}

// PageOf 根据列表构建分页结果，总数取列表长度，对应 PageResult.build(list)。
// 适用于不分页的全量查询要复用分页响应结构的场景。
func PageOf[T any](rows []T) PageResult[T] {
	rows = emptyIfNil(rows)
	return PageResult[T]{Total: int64(len(rows)), Rows: rows}
}

// EmptyPage 构建空分页结果，对应 PageResult.build()。
func EmptyPage[T any]() PageResult[T] {
	return PageResult[T]{Total: 0, Rows: []T{}}
}

// emptyIfNil 空切片兜底，保证序列化为 [] 而不是 null，对应 PageResult.emptyIfNull。
func emptyIfNil[T any](rows []T) []T {
	if rows == nil {
		return []T{}
	}
	return rows
}
