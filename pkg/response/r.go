// Package response 统一 HTTP 响应结构与快捷构造方法。
//
// 对应原项目 ruoyi-common-core 的 domain.R 与 domain.PageResult：
//   - R[T]          → org.dromara.common.core.domain.R<T>
//   - PageResult[T] → org.dromara.common.core.domain.PageResult<T>
//
// 与原项目一致，业务状态码放在响应体 code 字段里，HTTP 状态码始终为 200，
// 由前端统一拦截 code 判断成败。
package response

// 业务状态码，对应原项目 constant.HttpStatus。
// 注意 CodeWarn=601 不是 HTTP 状态码，是原项目自定义的「警告」语义。
const (
	CodeSuccess      = 200 // 操作成功
	CodeBadRequest   = 400 // 参数错误（缺少、格式不匹配）
	CodeUnauthorized = 401 // 未授权
	CodeForbidden    = 403 // 访问受限，授权过期
	CodeNotFound     = 404 // 资源、服务未找到
	CodeFail         = 500 // 系统内部错误
	CodeWarn         = 601 // 系统警告消息
)

// 默认提示信息。
const (
	MsgSuccess = "操作成功"
	MsgFail    = "操作失败"
)

// R 统一响应体。
//
// Data 不加 omitempty：对齐原项目 Jackson 默认行为（未配置 NON_NULL），
// 无数据时序列化为 "data": null，字段恒定存在。
type R[T any] struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data T      `json:"data"`
}

// Ok 构建成功响应（带业务数据），对应 R.ok(data) / R.data(data)。
func Ok[T any](data T) R[T] {
	return restResult(data, CodeSuccess, MsgSuccess)
}

// OkVoid 构建无数据的成功响应，对应 R.ok()。
func OkVoid() R[any] {
	return restResult[any](nil, CodeSuccess, MsgSuccess)
}

// OkMsg 构建成功响应（自定义提示、无数据），对应 R.ok(msg)。
func OkMsg(msg string) R[any] {
	return restResult[any](nil, CodeSuccess, msg)
}

// OkMsgData 构建成功响应（自定义提示 + 业务数据），对应 R.ok(msg, data)。
func OkMsgData[T any](msg string, data T) R[T] {
	return restResult(data, CodeSuccess, msg)
}

// Fail 构建失败响应（自定义提示），对应 R.fail(msg)。
// 传空串时退化为默认提示，等价于 R.fail()。
func Fail(msg string) R[any] {
	if msg == "" {
		msg = MsgFail
	}
	return restResult[any](nil, CodeFail, msg)
}

// FailData 构建失败响应（自定义提示 + 业务数据），对应 R.fail(msg, data)。
func FailData[T any](msg string, data T) R[T] {
	if msg == "" {
		msg = MsgFail
	}
	return restResult(data, CodeFail, msg)
}

// FailCode 构建失败响应（指定状态码 + 提示），对应 R.fail(code, msg)。
func FailCode(code int, msg string) R[any] {
	return restResult[any](nil, code, msg)
}

// Warn 构建警告响应，对应 R.warn(msg)。
// 用于「业务上不允许但不算异常」的场景，如存在下级部门不允许删除。
func Warn(msg string) R[any] {
	return restResult[any](nil, CodeWarn, msg)
}

// WarnData 构建警告响应（带业务数据），对应 R.warn(msg, data)。
func WarnData[T any](msg string, data T) R[T] {
	return restResult(data, CodeWarn, msg)
}

// IsSuccess 判断响应是否成功。
func (r R[T]) IsSuccess() bool {
	return r.Code == CodeSuccess
}

// IsError 判断响应是否失败。
func (r R[T]) IsError() bool {
	return !r.IsSuccess()
}

// restResult 核心构建方法，对应 R.restResult。
func restResult[T any](data T, code int, msg string) R[T] {
	return R[T]{Code: code, Msg: msg, Data: data}
}
