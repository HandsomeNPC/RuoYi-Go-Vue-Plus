// Package errs 业务错误类型，对应原项目 ruoyi-common-core 的 exception 包。
//
// 只放「能直接回给前端」的业务错误。基础设施错误（DB、Redis 连不上等）
// 直接返回原始 error，由 middleware.Recover 兜底成「发生未知异常」并打日志。
//
// 包名用 errs 而非 errors，避免与标准库 errors 冲突 —— 两者常需同时导入
// （本包的 As/Is 判断就依赖标准库）。
package errs

import "fmt"

// ServiceError 业务异常，对应 exception.ServiceException。
//
// Code 为 0 表示「不指定业务码」，由 middleware.Recover 回落到
// response.CodeFail(500)，对齐 Java 侧 code 为 null 时走 R.fail(msg) 的分支。
//
// Detail 对应 detailMessage：只进日志、不回前端，用于放 SQL、上游报文
// 这类不该暴露给用户的调试信息。
type ServiceError struct {
	Code   int
	Msg    string
	Detail string
}

// Error 实现 error。返回 Msg，对齐 Java 覆写 getMessage() 只返回 message。
func (e *ServiceError) Error() string {
	return e.Msg
}

// New 构造业务异常（不指定业务码），对应 new ServiceException(message)。
func New(msg string) *ServiceError {
	return &ServiceError{Msg: msg}
}

// Newf 构造业务异常并格式化消息，对应 ServiceException(message, args...)。
//
// Java 用 hutool 的 StrFormatter 占位符 {}，这里改用 Go 的 fmt 动词 %s/%d —
// 没有为了对齐语法去实现一套 {} 解析，调用处按 Go 习惯写即可。
func Newf(format string, args ...any) *ServiceError {
	return &ServiceError{Msg: fmt.Sprintf(format, args...)}
}

// NewCode 构造带业务码的业务异常，对应 ServiceException(message, code)。
func NewCode(code int, msg string) *ServiceError {
	return &ServiceError{Code: code, Msg: msg}
}

// WithDetail 附加错误明细并返回自身，对应 setDetailMessage 的链式用法。
func (e *ServiceError) WithDetail(detail string) *ServiceError {
	e.Detail = detail
	return e
}
