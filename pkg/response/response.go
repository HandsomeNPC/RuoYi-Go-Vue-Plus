// Package response 统一 HTTP 响应结构与快捷方法。
package response

import "net/http"

// R 统一响应体。
type R[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data,omitempty"`
}

const (
	CodeSuccess      = http.StatusOK
	CodeFail         = http.StatusInternalServerError
	CodeUnauthorized = http.StatusUnauthorized
	CodeForbidden    = http.StatusForbidden

	MsgSuccess = "操作成功"
	MsgFail    = "操作失败"
)

// Ok 携带数据的成功响应。
func Ok[T any](data T) R[T] {
	return R[T]{Code: CodeSuccess, Msg: MsgSuccess, Data: data}
}

// OkMsg 自定义提示的成功响应。
func OkMsg(msg string) R[any] {
	return R[any]{Code: CodeSuccess, Msg: msg}
}

// Fail 失败响应。
func Fail(msg string) R[any] {
	return R[any]{Code: CodeFail, Msg: msg}
}

// FailCode 指定状态码的失败响应。
func FailCode(code int, msg string) R[any] {
	return R[any]{Code: code, Msg: msg}
}

// PageResult 分页结果。
type PageResult[T any] struct {
	Total int64  `json:"total"`
	Rows  []T    `json:"rows"`
	Code  int    `json:"code"`
	Msg   string `json:"msg"`
}

// Page 构造分页结果。
func Page[T any](rows []T, total int64) PageResult[T] {
	return PageResult[T]{Total: total, Rows: rows, Code: CodeSuccess, Msg: MsgSuccess}
}
